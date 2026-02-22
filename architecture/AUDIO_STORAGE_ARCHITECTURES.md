# Audio Storage Architecture

## Current Implementation: Timestamped Chunks (Sparse Storage)

WisBot uses timestamped sparse chunks for voice recording. Only actual audio data is stored — silence is implicit and reconstructed at save time.

### How It Works

1. Each read from a user's `AudioInStream` creates an `AudioChunk` with:
   - `TimestampMs` — milliseconds since recording started
   - `Data` — raw PCM bytes (variable length)
   - Computed `FrameCount` and `DurationMs`

2. Per-user state lives in `UserAudio`: userId, username, chunk list, stream reference, recording task.

3. On stop, `ReconstructAudio()` allocates a zero-filled byte array for the full session duration and overlays each chunk at its timestamp position. Gaps become silence automatically.

### Why This Approach

- ~90% less memory than continuous storage (most time in voice channels is silence)
- No backfill or forward-fill logic needed for users joining/leaving mid-session
- No continuous 20ms write loop burning CPU
- Scales well for longer recordings with intermittent speech

### Trade-offs

- Reconstruction adds a processing step at save time
- Timestamp accuracy depends on `DateTime.UtcNow` granularity
- All chunks held in memory until save (no disk flush yet)

## Alternative Approaches Considered

### Continuous Timeline
Write a frame every 20ms (real audio or silence). Simple but memory-heavy — a 5-minute recording with 5 users uses ~288 MB.

### Segment-Based
Divide into fixed time segments (e.g., 10 seconds). Good for progressive saving, but segment boundaries can split audio awkwardly.

### Event-Based with VAD
Store audio only when voice activity is detected. Most memory-efficient but requires threshold tuning and can clip quiet speech.

### Stream to Disk
Write directly to temp files during recording. Minimal memory, good crash recovery, but adds disk I/O overhead and SSD wear.

### Hybrid (Memory + Disk)
Buffer in memory, flush to disk periodically. Best reliability and performance balance but most complex to implement.

## Comparison

| Approach | Memory | Complexity | Crash Recovery | Status |
|----------|--------|------------|----------------|--------|
| Timestamped Chunks | Low | Medium | None | **Current** |
| Continuous Timeline | High | Low | None | Replaced |
| Segment-Based | Medium | Medium | Good | Not implemented |
| Event-Based (VAD) | Very Low | High | None | Not implemented |
| Stream to Disk | Very Low | Low | Excellent | Not implemented |
| Hybrid | Low | Very High | Good | Not implemented |

## Potential Upgrades

If recordings get longer or memory becomes an issue, the next step would be adding periodic disk flushing (hybrid approach) on top of the existing timestamped chunk model.
