using Discord;
using Discord.Audio;
using Discord.WebSocket;
using NAudio.Wave;
using System.Collections.Concurrent;


/// Represents a chunk of audio data with its timestamp relative to recording start
public class AudioChunk {

    /// Timestamp in milliseconds from recording start
    public long TimestampMs { get; set; }

    /// Raw PCM audio data (48kHz, 16-bit, stereo)
    /// Variable length - typically one or more 20ms frames (3840 bytes each)
    public byte[] Data { get; set; } = Array.Empty<byte>();


    /// Number of 20ms frames this chunk contains
    public int FrameCount => Data.Length / 3840;


    /// Duration of this chunk in milliseconds
    public double DurationMs => FrameCount * 20.0;
}

/// All per-user recording state in one place
public class UserAudio {
    public ulong UserId { get; }
    public string Username { get; set; }
    public List<AudioChunk> Chunks { get; } = new();
    public AudioInStream? Stream { get; set; }
    public Task? RecordingTask { get; set; }

    public UserAudio(ulong userId, string username) {
        UserId = userId;
        Username = username;
    }
}

/// VoiceRecorder - Records audio from Discord voice channels
///
/// Uses IAudioClient.GetStreams() to receive audio from individual users.
/// Each user's audio is captured separately and saved as individual WAV files.
///
/// Note: Audio streams may take a moment to become available after joining.
/// The bot needs proper voice permissions and users must be actively speaking.

public class VoiceRecorder {
    private readonly Terminal _terminal;
    private IAudioClient? _audioClient;
    private bool _isRecording = false;
    private CancellationTokenSource? _recordingCancellationToken;

    // All per-user state in one dictionary: UserID -> UserAudio
    private ConcurrentDictionary<ulong, UserAudio> _users = new();

    // Track when recording started for synchronization
    private DateTime _recordingStartTime;

    // Voice channel being recorded
    private IVoiceChannel? _recordingVoiceChannel;

    public VoiceRecorder(Terminal terminal) {
        _terminal = terminal;
    }

    private async Task Log(string msg) {
        await _terminal.AddLine($"[VoiceRecorder] {msg}");
    }



    /// Joins the specified voice channel and starts recording all users in it.
    /// Used by slash command - requires SocketGuildUser get voice channel
    public async Task<string> JoinAndRecordUser(SocketGuildUser user) {
        if (user.VoiceChannel == null) {
            return "You must be in a voice channel first!";
        }

        return await JoinAndRecordChannel(user.VoiceChannel);
    }

    /// Joins a voice channel by ID and starts recording all users in it.
    /// Also used by terminal test commands.
    public async Task<string> JoinAndRecordChannel(IVoiceChannel voiceChannel) {
        if (_isRecording) {
            return "Already recording in a voice channel!";
        }

        try {
            _audioClient = await voiceChannel.ConnectAsync();
            _isRecording = true;
            _recordingCancellationToken = new CancellationTokenSource();
            _recordingStartTime = DateTime.UtcNow;
            _recordingVoiceChannel = voiceChannel;

            _users.Clear();

            // Subscribe to IAudioClient events
            _audioClient.StreamCreated += OnStreamCreated;
            _audioClient.StreamDestroyed += OnStreamDestroyed;
            _audioClient.Disconnected += OnAudioDisconnected;

            var voiceUsers = (await voiceChannel.GetUsersAsync().FlattenAsync())
                .Where(u => !u.IsBot && u.VoiceChannel?.Id == voiceChannel.Id).ToList();
            await Log($"Found {voiceUsers.Count} user(s) in voice channel {voiceChannel.Name}");

            foreach (var voiceUser in voiceUsers) {
                var userAudio = new UserAudio(voiceUser.Id, voiceUser.Username);
                userAudio.RecordingTask = Task.Run(() => RecordUser(userAudio, _recordingCancellationToken.Token));
                _users.TryAdd(voiceUser.Id, userAudio);
            }

            return $"Joined {voiceChannel.Name} and started recording {voiceUsers.Count} user(s)!";
        } catch (Exception ex) {
            return $"Failed to join voice channel: {ex.Message}";
        }
    }

    /// Polls GetStreams() for a user's initial stream, then delegates to ReadStream.
    /// Used for users already in the channel when recording starts.
    private async Task RecordUser(UserAudio userAudio, CancellationToken cancellationToken) {
        if (_audioClient == null) return;

        await Log($"Starting recording for user {userAudio.Username}...");

        try {
            // Poll for the stream (may take a moment to appear after connecting)
            for (int i = 0; i < 50 && !cancellationToken.IsCancellationRequested; i++) {
                var streams = _audioClient.GetStreams();
                if (streams.TryGetValue(userAudio.UserId, out var stream)) {
                    userAudio.Stream = stream;
                    await Log($"Found audio stream for {userAudio.Username}");
                    break;
                }
                await Task.Delay(100, cancellationToken);
            }

            // ReadStream handles null stream gracefully (waits for OnStreamCreated)
            await ReadStream(userAudio, cancellationToken);
        } catch (OperationCanceledException) {
            // Expected when stopping
        } catch (Exception ex) {
            await Log($"Error recording {userAudio.Username}: {ex.Message}");
        }
    }

    /// Core read loop. Reads from userAudio.Stream (hot-swappable by events).
    /// Stores timestamped chunks. Handles null/broken streams gracefully.
    private async Task ReadStream(UserAudio userAudio, CancellationToken cancellationToken) {
        const int frameSize = 3840; // 20ms at 48kHz, 16-bit, stereo
        byte[] readBuffer = new byte[frameSize];

        while (!cancellationToken.IsCancellationRequested && _isRecording) {
            var stream = userAudio.Stream;
            if (stream == null) {
                await Task.Delay(200, cancellationToken);
                continue;
            }

            // Use a linked token with timeout to detect broken streams
            using var readTimeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            readTimeout.CancelAfter(TimeSpan.FromSeconds(5));

            int bytesReadSize;
            try {
                bytesReadSize = await stream.ReadAsync(readBuffer, 0, frameSize, readTimeout.Token);
            } catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested) {
                await Log($"Read timeout for {userAudio.Username} — stream may be broken");
                userAudio.Stream = null; // Mark broken; OnStreamCreated can replace it
                continue;
            }

            if (bytesReadSize > 0) {
                var chunk = new AudioChunk {
                    TimestampMs = (long)(DateTime.UtcNow - _recordingStartTime).TotalMilliseconds,
                    Data = new byte[bytesReadSize]
                };
                Buffer.BlockCopy(readBuffer, 0, chunk.Data, 0, bytesReadSize);
                userAudio.Chunks.Add(chunk);
            }
        }

        await Log($"Recording ended for {userAudio.Username}");
    }

    // ── IAudioClient Event Handlers ──────────────────────────────────────

    private Task OnStreamCreated(ulong userId, AudioInStream stream) {
        if (!_isRecording || _recordingCancellationToken == null) return Task.CompletedTask;

        // Existing user — just update their stream reference (ReadStream picks it up)
        if (_users.TryGetValue(userId, out var existing)) {
            existing.Stream = stream;
            return Task.CompletedTask;
        }

        // New user — resolve username, create UserAudio, start reading
        _ = Task.Run(async () => {
            try {
                string username = userId.ToString();
                if (_recordingVoiceChannel is SocketVoiceChannel socketChannel) {
                    var guildUser = socketChannel.Guild.GetUser(userId);
                    if (guildUser?.IsBot == true) return;
                    if (guildUser != null) username = guildUser.Username;
                }

                var userAudio = new UserAudio(userId, username) { Stream = stream };
                if (_users.TryAdd(userId, userAudio)) {
                    await Log($"New user joined: {username}");
                    userAudio.RecordingTask = Task.Run(() =>
                        ReadStream(userAudio, _recordingCancellationToken!.Token));
                }
            } catch (Exception ex) {
                await Log($"Error handling new stream for {userId}: {ex.Message}");
            }
        });

        return Task.CompletedTask;
    }

    private Task OnStreamDestroyed(ulong userId) {
        if (_users.TryGetValue(userId, out var userAudio)) {
            userAudio.Stream = null;
            _ = Log($"Stream destroyed for {userAudio.Username}");
        }
        return Task.CompletedTask;
    }

    private Task OnAudioDisconnected(Exception ex) {
        _ = Log($"Audio client disconnected: {ex?.Message ?? "Unknown"}");
        return Task.CompletedTask;
    }

    // ── Stop / Save Pipeline ─────────────────────────────────────────────

    /// Shared shutdown: captures duration, cancels tasks, unsubscribes events, disconnects.
    private async Task<long> StopRecording() {
        var sessionDurationMs = (long)(DateTime.UtcNow - _recordingStartTime).TotalMilliseconds;

        _isRecording = false;
        _recordingCancellationToken?.Cancel();

        try {
            var tasks = _users.Values
                .Where(u => u.RecordingTask != null)
                .Select(u => u.RecordingTask!)
                .ToArray();

            if (tasks.Length > 0) {
                var all = Task.WhenAll(tasks);
                if (await Task.WhenAny(all, Task.Delay(10_000)) != all)
                    await Log("Some recording tasks did not finish within 10s — proceeding");
            }
        } catch (Exception ex) {
            await Log($"Error waiting for tasks: {ex.Message}");
        } finally {
            _recordingCancellationToken?.Dispose();
            _recordingCancellationToken = null;
        }

        if (_audioClient != null) {
            _audioClient.StreamCreated -= OnStreamCreated;
            _audioClient.StreamDestroyed -= OnStreamDestroyed;
            _audioClient.Disconnected -= OnAudioDisconnected;
            await _audioClient.StopAsync();
            _audioClient = null;
        }

        _recordingVoiceChannel = null;
        await Log($"Recording stopped ({sessionDurationMs / 1000.0:F1}s)");
        return sessionDurationMs;
    }

    /// Converts sparse timestamped chunks into a continuous PCM byte array.
    /// Gaps between chunks are filled with silence (zero bytes).
    private byte[] ReconstructAudio(UserAudio userAudio, long sessionDurationMs) {
        int totalFrames = (int)(sessionDurationMs / 20);
        if (totalFrames <= 0) return Array.Empty<byte>();

        byte[] result = new byte[totalFrames * 3840]; // pre-zeroed = silence

        foreach (var chunk in userAudio.Chunks) {
            int byteOffset = (int)(chunk.TimestampMs / 20) * 3840;
            int bytesToCopy = Math.Min(chunk.Data.Length, result.Length - byteOffset);
            if (bytesToCopy > 0 && byteOffset >= 0)
                Buffer.BlockCopy(chunk.Data, 0, result, byteOffset, bytesToCopy);
        }

        return result;
    }

    /// Logs per-user stats, reconstructs audio, writes WAV files.
    private async Task<List<string>> SaveAllUsersAsWav(long sessionDurationMs) {
        var outputDir = Path.Combine(Directory.GetCurrentDirectory(), "recordings");
        Directory.CreateDirectory(outputDir);

        var filePaths = new List<string>();
        var timestamp = DateTime.Now.ToString("yyyyMMdd_HHmmss");

        foreach (var (userId, user) in _users) {
            var chunks = user.Chunks;

            // Log stats
            if (chunks.Count > 0) {
                long audioMs = chunks.Sum(c => (long)c.DurationMs);
                double dataMB = chunks.Sum(c => c.Data.Length) / (1024.0 * 1024.0);
                await Log($"  {user.Username}: {chunks.Count} chunks, {audioMs / 1000.0:F1}s audio, {dataMB:F2} MB");
            } else {
                await Log($"  {user.Username}: no audio captured");
                continue;
            }

            // Reconstruct and write
            byte[] pcm = ReconstructAudio(user, sessionDurationMs);
            if (pcm.Length == 0) continue;

            var filePath = Path.Combine(outputDir, $"{user.Username}_{timestamp}.wav");
            using (var writer = new WaveFileWriter(filePath, new WaveFormat(48000, 16, 2)))
                writer.Write(pcm, 0, pcm.Length);

            await Log($"Saved {Path.GetFileName(filePath)} ({pcm.Length / (1024.0 * 1024.0):F2} MB)");
            filePaths.Add(filePath);
        }

        return filePaths;
    }

    /// Mixes multiple WAV files into one by summing samples with clamping.
    private async Task MergeAudioFiles(List<string> filePaths, string outputDir) {
        var mergedPath = Path.Combine(outputDir, $"merged_{DateTime.Now:yyyyMMdd_HHmmss}.wav");
        var readers = new List<WaveFileReader>();

        try {
            foreach (var path in filePaths)
                readers.Add(new WaveFileReader(path));

            using var writer = new WaveFileWriter(mergedPath, new WaveFormat(48000, 16, 2));
            byte[] buffer = new byte[3840];
            bool hasData = true;

            while (hasData) {
                hasData = false;
                short[] mixed = new short[buffer.Length / 2];

                foreach (var reader in readers) {
                    int bytesRead = reader.Read(buffer, 0, buffer.Length);
                    if (bytesRead <= 0) continue;
                    hasData = true;

                    for (int i = 0; i < bytesRead / 2; i++)
                        mixed[i] = (short)Math.Clamp(mixed[i] + BitConverter.ToInt16(buffer, i * 2), short.MinValue, short.MaxValue);
                }

                if (hasData) {
                    byte[] mixedBytes = new byte[mixed.Length * 2];
                    for (int i = 0; i < mixed.Length; i++)
                        BitConverter.GetBytes(mixed[i]).CopyTo(mixedBytes, i * 2);
                    writer.Write(mixedBytes, 0, mixedBytes.Length);
                }
            }

            await Log($"Merged {filePaths.Count} files into {Path.GetFileName(mergedPath)}");
            filePaths.Clear();
            filePaths.Add(mergedPath);
        } catch (Exception ex) {
            await Log($"Error merging audio: {ex.Message}");
        } finally {
            foreach (var reader in readers)
                reader.Dispose();
        }
    }

    // ── Public API ───────────────────────────────────────────────────────

    public bool IsRecording => _isRecording;

    /// Stops recording, saves WAV files, optionally merges and sends to Discord channel.
    public async Task<List<string>> StopRecordingAndSave(ISocketMessageChannel channel, bool sendFiles = false, bool mergeAudio = false) {
        if (!_isRecording || _audioClient == null) {
            await channel.SendMessageAsync("Not currently recording!");
            return new List<string>();
        }

        var sessionMs = await StopRecording();
        var files = await SaveAllUsersAsWav(sessionMs);

        if (mergeAudio && files.Count > 1) {
            var outputDir = Path.Combine(Directory.GetCurrentDirectory(), "recordings");
            await MergeAudioFiles(files, outputDir);
        }

        if (sendFiles && files.Count > 0) {
            foreach (var file in files)
                await channel.SendFileAsync(file, $"Recording: {Path.GetFileName(file)}");
        }

        return files;
    }

    /// Stops recording and saves WAV files locally (no Discord channel needed).
    public async Task<List<string>> StopRecordingAndSave() {
        if (!_isRecording || _audioClient == null) {
            await Log("Not currently recording!");
            return new List<string>();
        }

        var sessionMs = await StopRecording();
        return await SaveAllUsersAsWav(sessionMs);
    }
}