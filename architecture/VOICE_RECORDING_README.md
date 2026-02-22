# Voice Recording Implementation

## How It Works

Discord.Net supports receiving audio from individual users through `IAudioClient.GetStreams()` and `StreamCreated`/`StreamDestroyed` events.

The voice recording implementation uses:
- `IAudioClient.GetStreams()` - Polls for initial audio streams per user
- `IAudioClient.StreamCreated` / `StreamDestroyed` - Event-driven stream lifecycle
- NAudio - For writing WAV files
- Timestamped sparse chunks - Only stores audio when users are speaking

## Commands

- `/recording action:Start` - Join your current voice channel and start recording
- `/recording action:Stop` - Stop recording and save individual audio files
  - Optional: `sendfile:True` - Send WAV files directly in Discord chat
  - Optional: `mergeaudio:True` - Merge all users into a single file

## How to Use

1. Join a voice channel
2. Use `/recording action:Start`
3. Talk (or have others talk) in the voice channel
4. Use `/recording action:Stop` when done
5. Bot saves separate WAV files per user to `./recordings/`

## Technical Details

### Audio Format
- Sample rate: 48kHz
- Bit depth: 16-bit
- Channels: 2 (stereo)
- Frame size: 20ms (3840 bytes per frame)

### Architecture: Timestamped Sparse Chunks

Audio is stored as timestamped chunks rather than a continuous timeline. This means:
- Only actual audio data is kept in memory (no silent frames stored)
- Each chunk records its timestamp relative to the recording start
- On stop, chunks are reconstructed into continuous PCM by placing them at their timestamp positions and filling gaps with silence

### Recording Flow

1. Bot joins the voice channel via `IAudioClient`
2. Initial streams discovered by polling `GetStreams()` (up to 5 seconds)
3. `StreamCreated` event handles users joining mid-recording (hot-swaps stream references)
4. `StreamDestroyed` event marks streams as null; `ReadStream` waits for replacement
5. Each user gets a dedicated `ReadStream` task that stores `AudioChunk` objects
6. Per-read 5-second timeout detects broken streams
7. On stop, a 10-second safety timeout ensures all tasks finish
8. `ReconstructAudio()` builds continuous PCM from sparse chunks
9. WAV files written per user; optionally merged and/or sent to Discord

### Key Classes

- `AudioChunk` - Timestamped audio data with computed `FrameCount` and `DurationMs`
- `UserAudio` - Per-user state: userId, username, chunks list, stream reference, recording task
- `VoiceRecorder` - Main recorder: join/record/stop/save/merge pipeline

### File Output

- Directory: `./recordings/`
- Per-user format: `{username}_{yyyyMMdd_HHmmss}.wav`
- Merged format: `merged_{yyyyMMdd_HHmmss}.wav`

## Potential Issues

### Stream Availability
Audio streams may not appear immediately after joining. The code polls for up to 5 seconds, then falls back to `StreamCreated` events.

### Speaking Detection
Discord only sends audio packets when users are actively speaking. Silent users produce no chunks.

### Broken Streams
If a stream stops responding (5s read timeout), it's marked null. A new `StreamCreated` event can replace it automatically.

### Permissions
The bot needs:
- View Channel
- Connect
- Speak (may be required even for receiving)

### Memory Usage
Audio chunks are stored in memory until recording stops. Long recordings with many active speakers can use significant RAM, though sparse storage reduces this compared to continuous approaches.

## Troubleshooting

**No audio captured:**
- Make sure users are actually speaking
- Check bot has proper voice permissions
- Ensure users aren't muted/deafened

**Empty files:**
- Audio streams might not have been created yet
- Users may not have spoken during recording

**Bot doesn't join:**
- Check voice channel permissions
- Ensure bot has "Connect" permission
- Verify you're in a voice channel when using the command

## Code Structure

```
VoiceRecorder.cs
  AudioChunk              - Timestamped audio data
  UserAudio               - Per-user recording state
  VoiceRecorder
    JoinAndRecordUser()    - Entry point from slash command
    JoinAndRecordChannel() - Connects to channel, sets up recording
    RecordUser()           - Polls for initial stream, delegates to ReadStream
    ReadStream()           - Core read loop with timeout handling
    OnStreamCreated()      - Handles new/replacement streams
    OnStreamDestroyed()    - Marks stream as null
    StopRecording()        - Cancels tasks, unsubscribes events, disconnects
    ReconstructAudio()     - Sparse chunks -> continuous PCM
    SaveAllUsersAsWav()    - Writes WAV files per user
    MergeAudioFiles()      - Sums samples from multiple WAVs into one
    StopRecordingAndSave() - Public API (two overloads: with/without Discord channel)
```
