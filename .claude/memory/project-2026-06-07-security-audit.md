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

**Low backlog — ~18/22 resolved across PRs #33/#35/#36/#44/#45 (2026-06-08 → 06-16):**
- #33: L-6 (TryParseDuration overflow guard), L-20 (recordings auto-delete), L-1 (N-time download
  `WISBOT_UPLOAD_MAX_DOWNLOADS`, default 0=unlimited; `download_count` col + 1h grace), wisllm history retention.
- #35: L-7 (/voicestats ephemeral + self-or-mod), L-21 (voice_activity retention + README disclosure),
  L-22 (/testrecord announces via JoinAndRecordChannel choke point), L-16 (atomic per-user /remind cap).
- #36: L-3/L-19 (MinIO loopback binds), L-13 (deploy-o11y action SHA-pin + permissions), L-17
  (provenance+SBOM on publish), `.github/dependabot.yml` added.
- #44: L-9 (central command→permission map in OnSlashCommandExecuted), L-8 (per-user /wisllm rate
  limit `WISLLM_RATE_LIMIT_PER_MINUTE` default 10; kept GLOBAL **by choice** to preserve DM usage).
- #45: L-15 (compact loads ≤500 newest rows), L-16 (bounded delivery via shared SemaphoreSlim(8)),
  L-2 logging (no prompt text logged), L-4 (covered by consent/retention/README).

**Deliberately NOT done (judgment calls, flagged in report):** wisllm/recordings at-rest encryption
(controlled-volume threat model); initial base-image/minio **digest pins** L-11/L-12/L-18 — need the
linux/amd64 manifest digests from a connected env (Dependabot now manages bumps); periodic VACUUM.

**Retention pattern:** uploads/recordings/wisllm-history/voice-activity each own an idempotent
`StartRetention()`/`StopRetention()` sweep wired in `Bot.OnReady`/`StopBot`, all env-configurable
(30d default; voice-activity 90d). **Central authz:** `commandPermissions` map in Bot.cs gates
commands at the router (Administrator implies all); only `/recording`→MoveMembers today.
