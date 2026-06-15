# WisBot Security Audit — Low Priority / Deferred

_Date: 2026-06-07 · Companion to [`2026-06-07-security-audit.md`](2026-06-07-security-audit.md) (critical/high/medium — work those first). These 22 low-severity findings are hardening and defense-in-depth: real, worth doing, but not urgent. None is independently exploitable for high impact in the one-guild deployment model._

## Themes

- **Mention safety, privacy/retention, and per-user quotas** recur here as the *lower-impact siblings* of medium findings — fixing M-2/M-4/M-5 well will naturally close several of these.
- **Supply-chain hygiene** (digest pinning, image signing, removing the unused dependency) — cheap, do as a batch.
- **Privacy/consent/retention** around voice and conversation data — partly policy/documentation, not just code.

---

## Web surface

### L-1 · `GET /u/{id}` routes unauthenticated and not rate-limited — ✅ RESOLVED
`CWE-307` · `WebService.cs` — 128-bit CSPRNG ids make enumeration infeasible (holds). Rate-limiting shipped in M-6 (the limiter covers all `/u` routes). **Resolved:** added a configurable N-time download limit — `WISBOT_UPLOAD_MAX_DOWNLOADS` (default 0 = unlimited); a link auto-expires (410 Gone + cleanup) after N downloads. Uploader revoke/delete affordance remains a possible future enhancement.

## Secrets & data-at-rest

### L-2 · `wisllm_history` stored unencrypted, retained indefinitely; prompt prefixes logged — ⚠️ PARTIAL
`CWE-312` · `Database.cs`, `WisLlmService.cs` — **Resolved (retention):** history now auto-deletes after `WISLLM_HISTORY_RETENTION_DAYS` (default 30) via a periodic sweep (also addresses L-15). **Still open:** at-rest encryption and the prompt-prefix logging (`log model/length only`) — left as future hardening.

### L-3 · MinIO `minioadmin/minioadmin` + S3/console ports published to host (dev compose) — ✅ RESOLVED
`CWE-798` · `compose.yaml` — **Resolved:** MinIO S3/console ports now bind `127.0.0.1` (not LAN-reachable), with a prominent dev-only-credentials comment. (Dupes L-19, also resolved.)

### L-4 · Voice recordings written as cleartext WAV, no consent gate / access-control note
`CWE-359` · `VoiceRecorder.cs:401-402` — plaintext on a mounted volume. **Fix:** in-channel start/stop notice (folds into H-1/H-2), document retention, treat the recordings volume as sensitive. (Related: L-20.)

## Injection / output trust

### L-5 · Reminder channel-fallback reflects user text beside a live mention, no `AllowedMentions`
`CWE-74` · `ReminderService.cs:77` — **this is the same code as medium M-4**; listed here from the injection dimension. Fix once: targeted `AllowedMentions` (only the reminder's user id).

### L-6 · `TryParseDuration` can throw unhandled `OverflowException` on huge numeric input — ✅ RESOLVED
`CWE-190` · `ReminderService.cs` — confirmed real (`/remind when:2147483647d`). **Resolved:** the unit conversions + accumulation are wrapped in `try/catch(OverflowException) → return false`, so an over-large duration is rejected as unparseable instead of crashing the handler.

## Discord authorization & privacy

### L-7 · `/voicestats` exposes any member's full voice history publicly to any member — ✅ RESOLVED
`CWE-359` · `VoiceStatsService.cs` — **Resolved:** the response is now ephemeral, and viewing another member's stats requires Administrator/Move Members (you can always view your own).

### L-8 · `/wisllm` is a GLOBAL command — usable in DMs and from any guild sharing the bot
`CWE-862` · `Bot.cs:280` — registered via `CreateGlobalApplicationCommandAsync` (others are guild-scoped); shared guild session also enables persistent prompt injection. **Fix:** register guild-scoped to `Config.GuildId` (or set context types / disable DM), add per-user rate limiting.

### L-9 · No central permission gating at the command router
`CWE-862` · `Bot.cs:295-302` — dispatch is name-only; no command sets `WithDefaultMemberPermissions`. **Fix:** a central authorization step in `OnSlashCommandExecuted` (per-command required-permission/role map vs `command.User` `GuildPermissions`) + `WithDefaultMemberPermissions` at registration. (H-1 is the urgent instance of this; this is the systemic fix.)

## Dependencies & supply chain

### L-10 · Unused `Concentus.Oggfile` / `Concentus` dependency
`CWE-1104` · `Wisbot.csproj:21` — zero references in any `.cs` (verified); pulls transitive `Concentus 2.2.1`. Supply-chain surface for nothing. **Fix:** delete the `PackageReference`, `dotnet build` to confirm, re-add only when Ogg/Opus encoding is actually implemented.

### L-11 / L-18 · Dockerfile base images use mutable tags, not digests — ⚠️ PARTIAL
`CWE-1357` · `Dockerfile:4-5,14-15` — `sdk:10.0` / `aspnet:10.0` are re-published on every patch. **Mechanism added:** `.github/dependabot.yml` now tracks the docker ecosystem so digest bumps get managed PRs. **Still open:** the initial `@sha256:` pin needs the linux/amd64 manifest digests (the arch CI builds for) — must come from `docker manifest inspect`, not a local arm64 cache; a network-connected follow-up.

### L-12 · `compose.yaml` uses `minio/minio:latest` — ⚠️ PARTIAL
`CWE-1357` · `compose.yaml` — local-dev only; mutable. **Mechanism added:** Dependabot (docker) tracks it. **Still open:** pinning to a specific `RELEASE.YYYY-...` tag needs a verified current tag (a network-connected follow-up); not hand-pinned to avoid a broken local-dev pull.

### L-13 · Disabled `deploy-o11y.yml` uses mutable `actions/checkout@v4` — ✅ RESOLVED
`CWE-1357` · `.github/workflows/deploy-o11y.yml` — **Resolved:** SHA-pinned `actions/checkout` (reusing the repo's existing pin) + added a `permissions: { contents: read }` block, matching the active workflows. (Left in place rather than deleted — the delete-vs-keep call belongs to the agent-cloud o11y migration, not this security pass.)

### L-17 · Published image has no signature / SBOM / provenance — ✅ RESOLVED
`CWE-345` · `build-and-publish.yml` — **Resolved:** the build-push step now sets `provenance: mode=max` + `sbom: true`, attaching SLSA provenance and an SBOM to the published image. (agent-cloud verifying the attestation before deploy is a platform-side follow-up.)

## Resource exhaustion (lower-impact quota gaps)

### L-14 · No per-user `/upload` quota (30-day retention)
`CWE-770` · `UploadService.cs:114-132` — **same root as medium M-5**; fix the quota once.

### L-15 · `wisllm_history` grows unbounded per session — ⚠️ PARTIAL
`CWE-770` · `WisLlmService.cs` — **Resolved (age):** the retention sweep (L-2, `WISLLM_HISTORY_RETENTION_DAYS` default 30) now bounds growth by age. **Still open:** `compact`'s full-history load has no `LIMIT`, and there's no periodic VACUUM — minor, left as future work.

### L-16 · No per-user reminder cap — ✅ RESOLVED (cap)
`CWE-770` · `ReminderService.cs` — **Resolved:** `/remind` rejects once a user has `WISBOT_REMINDER_MAX_PER_USER` (default 25) pending reminders. **Note:** the per-tick unbounded concurrent `Deliver` task spawn is a separate, minor concern left as future work.

## Deployment

### L-19 · MinIO default creds + ports on all interfaces (dev compose) — ✅ RESOLVED
`CWE-1392` · `compose.yaml` — **Resolved** with L-3: ports bound to `127.0.0.1` + dev-only-credentials comment.

## Voice privacy / retention

### L-20 · Recordings kept indefinitely (no cleanup) — ✅ RESOLVED
`CWE-359` · `VoiceRecorder.cs` — **Resolved:** an hourly sweep deletes saved `*.wav` files older than `WISBOT_RECORDINGS_RETENTION_DAYS` (default 30), wired into `Bot.OnReady`/`StopBot`.

### L-21 · Passive `UserVoiceActivityTracker` logs every join/leave forever — a standing surveillance DB — ✅ RESOLVED
`CWE-359` · `UserVoiceActivityTracker.cs` — **Resolved:** a 12-hourly sweep deletes rows past `WISBOT_VOICE_ACTIVITY_RETENTION_DAYS` (default 90), the README Data section now discloses the passive logging, and `/voicestats` is restricted (L-7).

### L-22 · `/testrecord` silently records a hardcoded channel with no in-guild notice — ✅ RESOLVED
`CWE-359` · `VoiceRecorder.cs` — **Resolved:** the in-voice-channel "recording started" disclosure moved into the shared `JoinAndRecordChannel` choke point, so `/testrecord` (and any future entry point) announces too.

---

## Refuted during verification (recorded for transparency — NOT action items)

The adversarial verifiers rejected these as not exploitable / mis-attributed for this deployment:

- Multipart **filename reflected into `Content-Disposition`** without app-side validation — ASP.NET Core encodes it and rejects CRLF; the existing attachment-disposition defense holds.
- **Discord.Net internal log** forwarded to stdout without a level cap — not a leak.
- **Discord.Net 3.19.0-beta.1 beta-in-production** — it's the documented requirement for `IAudioClient.GetStreams()`; functional risk, not a security vuln (tracked in the main report's dependency note).
- **Unpinned apt packages** in the runtime image — marginal for a rebuilt-on-merge image.
- **No per-user `/remind` cap** as a *DM-spam* vector — reminders DM the *scheduler*, not others; self-spam only (the DB-growth angle survives as L-16).
- **`/notify` covert presence tracking** — the deep link/DM only fires on a join the watcher could already observe; no new disclosure.
- **PR-triggered Docker build sharing GHA cache with the publish build** — publish is push-to-main only; PR builds don't feed `:latest`.
- **Global Kestrel `MaxRequestBodySize` affecting `/health`** — GET `/health` has no body; no impact.
