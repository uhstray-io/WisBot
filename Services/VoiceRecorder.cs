using Discord;
using Discord.Audio;
using Discord.WebSocket;
using NAudio.Wave;
using System.Collections.Concurrent;

namespace WisBot;

/// Represents a chunk of audio data with its timestamp relative to recording start.
/// Immutable after construction — use init-only properties with object initializer syntax.
public record AudioChunk {
    /// Timestamp in milliseconds from recording start
    public required long TimestampMs { get; init; }

    /// Raw PCM audio data (48kHz, 16-bit, stereo)
    /// Variable length - typically one or more 20ms frames (3840 bytes each)
    public required byte[] Data { get; init; }

    /// Number of 20ms frames this chunk contains
    public int FrameCount => Data.Length / 3840;

    /// Duration of this chunk in milliseconds
    public double DurationMs => FrameCount * 20.0;
}

/// All per-user recording state in one place
public class UserAudio(ulong userId, string username) {
    public ulong UserId { get; } = userId;
    public string Username { get; set; } = username;
    public List<AudioChunk> Chunks { get; } = [];
    public AudioInStream? Stream { get; set; }
    public Task? RecordingTask { get; set; }
}

/// VoiceRecorder - Records audio from Discord voice channels
///
/// Uses IAudioClient.GetStreams() to receive audio from individual users.
/// Each user's audio is captured separately and saved as individual WAV files.
///
/// Note: Audio streams may take a moment to become available after joining.
/// The bot needs proper voice permissions and users must be actively speaking.

public class VoiceRecorder(Terminal terminal) {
    private IAudioClient? audioClient;
    // 0 = idle, 1 = recording. Use Interlocked for thread-safe check-and-set —
    // a plain bool would allow two concurrent /recording start calls to both pass
    // the "already recording?" check before either sets the flag.
    private int isRecordingFlag = 0;
    private CancellationTokenSource? recordingCancellationToken;

    // All per-user state in one dictionary: UserID -> UserAudio
    private ConcurrentDictionary<ulong, UserAudio> users = new();

    // Track when recording started for synchronization
    private DateTime recordingStartTime;

    // Voice channel being recorded
    private IVoiceChannel? recordingVoiceChannel;

    private async Task Log(string msg, LogLevel level = LogLevel.Info) =>
        await terminal.AddLine($"[VoiceRecorder] {msg}", level);



    // ── Command ──────────────────────────────────────────────────────────

    public async Task HandleRecordingCommand(SocketSlashCommand command) {
        var actionOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "action");
        var sendFileOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "sendfile");
        var mergeAudioOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "mergeaudio");

        if (actionOption == null) {
            await command.RespondAsync("Action parameter is required!");
            return;
        }

        var action = (string)actionOption.Value!;
        var sendFile = sendFileOption != null && (bool)sendFileOption.Value!;
        var mergeAudio = mergeAudioOption != null && (bool)mergeAudioOption.Value!;

        _ = Log($"Recording command received with action: {action}, sendfile: {sendFile}, mergeaudio: {mergeAudio}");

        if (action == "start") {
            var user = command.User as SocketGuildUser;
            if (user?.VoiceChannel == null) {
                await command.RespondAsync("You must be in a voice channel to use this command!");
                return;
            }
            await command.RespondAsync("Joining voice channel and starting recording...");
            _ = Task.Run(async () => {
                try {
                    var result = await JoinAndRecordUser(user);
                    await Log($"Finished recording for {user.Username}. Result: {result}");
                    await command.FollowupAsync(result);
                } catch (Exception ex) {
                    await Log($"Error in recording task: {ex.Message}", LogLevel.Error);
                    await command.FollowupAsync($"Error starting recording: {ex.Message}");
                }
            });
            return;
        }

        if (action == "stop") {
            await command.RespondAsync("Stopping recording and processing audio files...");
            _ = Task.Run(async () => {
                try {
                    var result = await StopRecordingAndSave(command.Channel, sendFiles: sendFile, mergeAudio: mergeAudio);
                    await Log($"Finished processing audio files. Saved {result.Count} file(s).");
                    if (result.Count > 0) {
                        if (sendFile) {
                            await command.FollowupAsync(mergeAudio
                                ? $"✅ Recording saved and sent! Merged {result.Count} user(s) into 1 file."
                                : $"✅ Recording saved and sent! Processed {result.Count} user(s).");
                        } else {
                            await command.FollowupAsync(mergeAudio
                                ? $"✅ Recording saved locally! Merged {result.Count} user(s) into 1 file. Use a file link service to share (coming soon)."
                                : $"✅ Recording saved locally! Processed {result.Count} user(s). Use a file link service to share (coming soon).");
                        }
                    } else {
                        await command.FollowupAsync("⚠️ No audio was captured during the recording session.");
                    }
                } catch (Exception ex) {
                    await Log($"Error processing recording: {ex.Message}", LogLevel.Error);
                    await command.FollowupAsync($"❌ Error processing recording: {ex.Message}");
                }
            });
            return;
        }

        await command.RespondAsync("Invalid action. Use 'start' or 'stop'.");
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
        // Atomically set flag from 0 → 1. If it was already 1, bail immediately.
        if (Interlocked.CompareExchange(ref isRecordingFlag, 1, 0) == 1)
            return "Already recording in a voice channel!";

        try {
            audioClient = await voiceChannel.ConnectAsync();
            recordingCancellationToken = new CancellationTokenSource();
            recordingStartTime = DateTime.UtcNow;
            recordingVoiceChannel = voiceChannel;

            users.Clear();

            // Subscribe to IAudioClient events
            audioClient.StreamCreated += OnStreamCreated;
            audioClient.StreamDestroyed += OnStreamDestroyed;
            audioClient.Disconnected += OnAudioDisconnected;

            var voiceUsers = (await voiceChannel.GetUsersAsync().FlattenAsync())
                .Where(u => !u.IsBot && u.VoiceChannel?.Id == voiceChannel.Id).ToList();
            await Log($"Found {voiceUsers.Count} user(s) in voice channel {voiceChannel.Name}");

            foreach (var voiceUser in voiceUsers) {
                var userAudio = new UserAudio(voiceUser.Id, voiceUser.Username);
                userAudio.RecordingTask = Task.Run(() => RecordUser(userAudio, recordingCancellationToken.Token));
                users.TryAdd(voiceUser.Id, userAudio);
            }

            return $"Joined {voiceChannel.Name} and started recording {voiceUsers.Count} user(s)!";
        } catch (Exception ex) {
            return $"Failed to join voice channel: {ex.Message}";
        }
    }

    /// Polls GetStreams() for a user's initial stream, then delegates to ReadStream.
    /// Used for users already in the channel when recording starts.
    private async Task RecordUser(UserAudio userAudio, CancellationToken cancellationToken) {
        if (audioClient == null) return;

        await Log($"Starting recording for user {userAudio.Username}...");

        try {
            // Poll for the stream (may take a moment to appear after connecting)
            for (int i = 0; i < 50 && !cancellationToken.IsCancellationRequested; i++) {
                var streams = audioClient.GetStreams();
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
            await Log($"Error recording {userAudio.Username}: {ex.Message}", LogLevel.Error);
        }
    }

    /// Core read loop. Reads from userAudio.Stream (hot-swappable by events).
    /// Stores timestamped chunks. Handles null/broken streams gracefully.
    private async Task ReadStream(UserAudio userAudio, CancellationToken cancellationToken) {
        const int frameSize = 3840; // 20ms at 48kHz, 16-bit, stereo
        byte[] readBuffer = new byte[frameSize];

        while (!cancellationToken.IsCancellationRequested && isRecordingFlag == 1) {
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
                    TimestampMs = (long)(DateTime.UtcNow - recordingStartTime).TotalMilliseconds,
                    Data = new byte[bytesReadSize]
                };
                Buffer.BlockCopy(readBuffer, 0, chunk.Data, 0, bytesReadSize);
                userAudio.Chunks.Add(chunk);
            }
        }

        await Log($"Recording ended for {userAudio.Username}");
    }

    // ── IAudioClient Events ──────────────────────────────────────────────

    private Task OnStreamCreated(ulong userId, AudioInStream stream) {
        if (isRecordingFlag == 0 || recordingCancellationToken == null) return Task.CompletedTask;

        // Existing user — just update their stream reference (ReadStream picks it up)
        if (users.TryGetValue(userId, out var existing)) {
            existing.Stream = stream;
            return Task.CompletedTask;
        }

        // New user — resolve username, create UserAudio, start reading
        _ = Task.Run(async () => {
            try {
                string username = userId.ToString();
                if (recordingVoiceChannel is SocketVoiceChannel socketChannel) {
                    var guildUser = socketChannel.Guild.GetUser(userId);
                    if (guildUser?.IsBot == true) return;
                    if (guildUser != null) username = guildUser.Username;
                }

                var userAudio = new UserAudio(userId, username) { Stream = stream };
                if (users.TryAdd(userId, userAudio)) {
                    await Log($"New user joined: {username}");
                    userAudio.RecordingTask = Task.Run(() =>
                        ReadStream(userAudio, recordingCancellationToken!.Token));
                } else if (users.TryGetValue(userId, out var raced)) {
                    // Lost a concurrent-create race — hand the stream to the winner
                    // instead of leaking it unread.
                    raced.Stream = stream;
                }
            } catch (Exception ex) {
                await Log($"Error handling new stream for {userId}: {ex.Message}", LogLevel.Error);
            }
        });

        return Task.CompletedTask;
    }

    private Task OnStreamDestroyed(ulong userId) {
        if (users.TryGetValue(userId, out var userAudio)) {
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
        var sessionDurationMs = (long)(DateTime.UtcNow - recordingStartTime).TotalMilliseconds;

        Interlocked.Exchange(ref isRecordingFlag, 0);
        recordingCancellationToken?.Cancel();

        try {
            var tasks = users.Values
                .Where(u => u.RecordingTask != null)
                .Select(u => u.RecordingTask!)
                .ToArray();

            if (tasks.Length > 0) {
                var all = Task.WhenAll(tasks);
                if (await Task.WhenAny(all, Task.Delay(10_000)) != all)
                    await Log("Some recording tasks did not finish within 10s — proceeding");
            }
        } catch (Exception ex) {
            await Log($"Error waiting for tasks: {ex.Message}", LogLevel.Error);
        } finally {
            recordingCancellationToken?.Dispose();
            recordingCancellationToken = null;
        }

        if (audioClient != null) {
            audioClient.StreamCreated -= OnStreamCreated;
            audioClient.StreamDestroyed -= OnStreamDestroyed;
            audioClient.Disconnected -= OnAudioDisconnected;
            await audioClient.StopAsync();
            audioClient = null;
        }

        recordingVoiceChannel = null;
        await Log($"Recording stopped ({sessionDurationMs / 1000.0:F1}s)");
        return sessionDurationMs;
    }

    /// Converts sparse timestamped chunks into a continuous PCM byte array.
    /// Gaps between chunks are filled with silence (zero bytes).
    private byte[] ReconstructAudio(UserAudio userAudio, long sessionDurationMs) {
        // Snapshot first: if the 10s stop-drain timed out, a ReadStream task may still
        // be appending — enumerate a stable copy, not the live list.
        AudioChunk[] chunks = [.. userAudio.Chunks];

        // sessionDurationMs is snapshotted before the stop drain, but ReadStream keeps
        // appending chunks for up to 10s after — size the buffer to whichever ends last
        // so drain-window audio isn't silently dropped.
        long effectiveMs = sessionDurationMs;
        foreach (var chunk in chunks)
            effectiveMs = Math.Max(effectiveMs, chunk.TimestampMs + (long)Math.Ceiling(chunk.DurationMs));

        int totalFrames = (int)(effectiveMs / 20);
        if (totalFrames <= 0) return [];

        byte[] result = new byte[totalFrames * 3840]; // pre-zeroed = silence

        foreach (var chunk in chunks) {
            int byteOffset = (int)(chunk.TimestampMs / 20) * 3840;
            int bytesToCopy = Math.Min(chunk.Data.Length, result.Length - byteOffset);
            if (bytesToCopy > 0 && byteOffset >= 0)
                Buffer.BlockCopy(chunk.Data, 0, result, byteOffset, bytesToCopy);
        }

        return result;
    }

    /// Discord usernames may contain path separators or filesystem-invalid characters.
    private static string SanitizeFileName(string name) {
        var invalid = Path.GetInvalidFileNameChars();
        var cleaned = new string([.. name.Select(c => invalid.Contains(c) ? '_' : c)]).Trim();
        return cleaned.Length > 0 ? cleaned : "user";
    }

    /// Logs per-user stats, reconstructs audio, writes WAV files.
    private async Task<List<string>> SaveAllUsersAsWav(long sessionDurationMs) {
        var outputDir = Path.Combine(Directory.GetCurrentDirectory(), Config.RecordingsDir);
        Directory.CreateDirectory(outputDir);

        List<string> filePaths = [];
        var timestamp = DateTime.UtcNow.ToString("yyyyMMdd_HHmmss");

        foreach (var (userId, user) in users) {
            var chunks = user.Chunks.ToArray(); // snapshot — tasks may still append after a drain timeout

            // Log stats
            if (chunks.Length > 0) {
                long audioMs = chunks.Sum(c => (long)c.DurationMs);
                double dataMB = chunks.Sum(c => c.Data.Length) / (1024.0 * 1024.0);
                await Log($"  {user.Username}: {chunks.Length} chunks, {audioMs / 1000.0:F1}s audio, {dataMB:F2} MB");
            } else {
                await Log($"  {user.Username}: no audio captured");
                continue;
            }

            // Reconstruct and write. Per-user try/catch: one bad write must not drop
            // the rest of the session's recordings.
            try {
                byte[] pcm = ReconstructAudio(user, sessionDurationMs);
                if (pcm.Length == 0) continue;

                var filePath = Path.Combine(outputDir, $"{SanitizeFileName(user.Username)}_{timestamp}.wav");
                using (var writer = new WaveFileWriter(filePath, new WaveFormat(48000, 16, 2)))
                    writer.Write(pcm, 0, pcm.Length);

                await Log($"Saved {Path.GetFileName(filePath)} ({pcm.Length / (1024.0 * 1024.0):F2} MB)");
                filePaths.Add(filePath);
            } catch (Exception ex) {
                await Log($"Failed to save recording for {user.Username}: {ex.Message}", LogLevel.Error);
            }
        }

        return filePaths;
    }

    /// Mixes multiple WAV files into one by summing samples with clamping.
    private async Task MergeAudioFiles(List<string> filePaths, string outputDir) {
        var mergedPath = Path.Combine(outputDir, $"merged_{DateTime.UtcNow:yyyyMMdd_HHmmss}.wav");
        List<WaveFileReader> readers = [];

        try {
            foreach (var path in filePaths) {
                var reader = new WaveFileReader(path);
                // The mixer sums raw interleaved Int16 — a differently-formatted input
                // would be garbage. All our writers emit 48k/16/2; guard the invariant.
                if (reader.WaveFormat.SampleRate != 48000 || reader.WaveFormat.BitsPerSample != 16
                    || reader.WaveFormat.Channels != 2) {
                    await Log($"Skipping {Path.GetFileName(path)} in merge — unexpected format {reader.WaveFormat}", LogLevel.Warn);
                    reader.Dispose();
                    continue;
                }
                readers.Add(reader);
            }

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
            await Log($"Error merging audio: {ex.Message}", LogLevel.Error);
        } finally {
            foreach (var reader in readers)
                reader.Dispose();
        }
    }

    // ── Public API ───────────────────────────────────────────────────────

    public bool IsRecording => isRecordingFlag == 1;

    /// Stops recording, saves WAV files, optionally merges and sends to Discord channel.
    public async Task<List<string>> StopRecordingAndSave(ISocketMessageChannel channel, bool sendFiles = false, bool mergeAudio = false) {
        if (isRecordingFlag == 0 || audioClient == null) {
            await channel.SendMessageAsync("Not currently recording!");
            return [];
        }

        var sessionMs = await StopRecording();
        var files = await SaveAllUsersAsWav(sessionMs);

        if (mergeAudio && files.Count > 1) {
            var outputDir = Path.Combine(Directory.GetCurrentDirectory(), Config.RecordingsDir);
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
        if (isRecordingFlag == 0 || audioClient == null) {
            await Log("Not currently recording!");
            return [];
        }

        var sessionMs = await StopRecording();
        return await SaveAllUsersAsWav(sessionMs);
    }
}
