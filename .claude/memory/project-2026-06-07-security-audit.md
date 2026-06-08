---
name: project-2026-06-07-security-audit
description: Outcome of the 2026-06-07 multi-agent security audit — all high/medium fixed; reports + low backlog
metadata:
  node_type: memory
  type: project
---

73-agent, 8-dimension adversarially-verified security audit (2026-06-07). Result: **0 critical,
2 high, 7 medium, 22 low** (8 raw findings refuted by verification). Reports live at
`docs/security/2026-06-07-security-audit.md` (high/med) and `…-low-priority.md` (deferred backlog
+ refuted record). All high+medium remediated in severity-ordered PRs **#27 (H-1/H-2/M-3),
#28 (M-1/M-2/M-4), #29 (M-5/M-6/M-7), #30 (deps)**. No vulnerable NuGet packages (verified).
See also the earlier comprehensive review: [[project-2026-06-05-review-fixes]].

**New invariants / behaviors (don't regress):**
- `/recording` is gated by `GuildPermission.MoveMembers`, enforced **server-side** in
  `HandleRecordingCommand` (DefaultMemberPermissions alone is admin-overridable). Start/stop post
  a **non-ephemeral** consent notice; the "recording started" notice fires only AFTER join succeeds.
- Voice capture auto-stops at `WISBOT_RECORDING_MAX_MINUTES` (120) / `WISBOT_RECORDING_MAX_BYTES`
  (2 GB) — keeps the session alive + connection attached and freezes the duration (don't flip to
  idle: that breaks `/recording stop`).
- All bot-originated messages carrying user/model text use `AllowedMentions.None` (or a single
  explicit UserId for the reminder fallback). Never echo model output with default mentions.
- Never log raw Discord message content (metadata only) — privileged MessageContent intent.
- Upload POST claims the single-use link BEFORE buffering the body; per-user quota
  (`WISBOT_UPLOAD_MAX_LINKS_PER_USER` 20 / `…_MAX_BYTES_PER_USER` 2 GB) + per-IP rate limit
  (`WISBOT_UPLOAD_RATE_LIMIT_PER_MINUTE` 30, X-Forwarded-For). Default per-file cap is now **100 MB**.
- Discord.Net held at **3.19.0-beta.1** on purpose (voice `GetStreams()` APIs); a 3.20.0-stable
  bump needs a voice smoke test first. Concentus.Oggfile removed (was unused).

**Low backlog — partially worked (PR #33, 2026-06-08):** L-6 (TryParseDuration overflow guard),
L-20 (recordings auto-delete, `WISBOT_RECORDINGS_RETENTION_DAYS` default 30), L-1 (N-time download
limit `WISBOT_UPLOAD_MAX_DOWNLOADS` default 0=unlimited; `download_count` column + 1h grace before
cleanup), and WisLLM history retention (`WISLLM_HISTORY_RETENTION_DAYS` default 30, sweep also purges
orphaned sessions) → resolved. **Still deferred:** wisllm_history at-rest encryption + `compact`
LIMIT (L-2/L-15 partial); supply-chain digest pinning + image signing; passive
`UserVoiceActivityTracker` forever-DB; per-user `/remind` cap; central command authz; `/wisllm` is
GLOBAL (should be guild-scoped); MinIO dev-compose loopback binds. Full status in the low-priority report.

**Retention pattern:** uploads/recordings/wisllm-history each own a `StartRetention()`/`StopRetention()`
sweep (idempotent, wired in `Bot.OnReady`/`StopBot`), all default 30 days, all env-configurable.
