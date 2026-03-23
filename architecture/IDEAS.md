  Simple / Standard

  1. /ping / status command — uptime, latency, memory usage. Every bot has it, trivial to add using Environment.TickCount64 and GC.GetTotalMemory().
  2. Welcome messages — hook UserJoined to send a configurable greeting to a designated channel when someone joins the server. Natural extension of the MOTD config
  system already in place.
  3. /remind — one-shot reminders — user sets a message and a duration (/remind 2h Stand up meeting), bot DMs or replies at that time. The scheduler loop from MOTD is
  already there; reminders are just single-fire entries.
  4. Bulk message purge — /purge <count> deletes the last N messages from a channel. Common moderation tool, straightforward with Discord.Net's DeleteMessagesAsync.
  5. Reaction roles — bot posts a message; users react to get/remove a role. Very common, pairs well with the existing guild-scoped command pattern.

  ---
  Medium / Builds on Existing Features

  6. Voice presence notifications — ping a user (or a channel) when a specific person joins a voice channel. Hook UserVoiceStateUpdated, configurable via slash
  command. Pairs naturally with the recording system.
  7. /recording clip — retroactive capture — the bot sits silently in a voice channel buffering a rolling window (e.g. last 60 seconds). /recording clip saves that
  window as a WAV. The circular buffer is a small change to VoiceRecorder.
  8. Scheduled announcements — generalize the MOTD scheduler to support arbitrary recurring messages with cron-like recurrence (daily, weekly, specific days). The
  scheduler data model just needs a RecurrencePattern field.
  9. Voice session log — post a summary to a channel when a recording stops: who was in the session, how long, how much audio each person contributed. The data is
  already computed in SaveAllUsersAsWav's stat logging — just send it to Discord instead of the console.
  10. /quote add/random/search — a server quote book. Users save memorable lines with attribution. Stored as JSON like the MOTD config. /quote random pulls one. Pairs
  nicely with a daily quote scheduled via MOTD-style delivery.

  ---
  Creative / Unique

  11. Auto meeting notes from recordings — after /recording stop, pipe the WAV files to a local or API-hosted Whisper instance for transcription, then feed the
  transcript to the /wisllm LLM endpoint (already stubbed) to produce a bullet-point summary posted back to Discord. The whole pipeline is already shaped for it.
  12. Voice activity heatmap — since AudioChunk timestamps are already stored, compute a per-user silence/speech breakdown across the session and post it as a text bar
   chart. Shows who dominated the call and when, with no extra recording overhead.
  13. "Time capsule" messages — /timecapsule <date> <message> stores a message to be delivered on a future date — weeks or months out. The scheduler already handles
  date comparison against LastSentDate; this is the inverse. Good for anniversary announcements, end-of-year reflections, bets.
  14. GitHub/webhook event feed — open an HTTP listener (minimal HttpListener or ASP.NET minimal API) that accepts GitHub push/PR webhooks and formats them into a
  configured Discord channel. Turns the bot into a lightweight CI notifier without a separate service.
  15. Server "newspaper" — once a day (using the MOTD scheduler), aggregate: most active text channel, most active voice user, any new members, a random quote, and
  post it as a formatted digest. All the data comes from events the bot already observes — it just needs to accumulate counters in memory.
  16. Ambient voice recording mode — the bot permanently sits in a voice channel and maintains a rolling 24-hour sparse audio archive on disk. On demand, any time
  window can be reconstructed into a WAV. Effectively a "DVR for your Discord server." The sparse AudioChunk architecture in VoiceRecorder was essentially built for
  this — it just needs rotation/pruning logic.
  17. Per-user voice analytics over time — persist session stats (duration, speaking time ratio, participants) to a JSON log after each recording. Over weeks, /stats
  @user can show trends: average session length, most frequent co-participants, time-of-day patterns. A lightweight alternative to a full database given the project's
  file-based persistence style.