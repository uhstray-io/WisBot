# WisBot Security Audit — Critical / High / Medium

_Date: 2026-06-07 · Method: 8-dimension parallel multi-agent audit (73 agents), every finding adversarially verified (3-vote panels for high, 2 for medium). Low-severity and deferred items are in [`2026-06-07-security-low-priority.md`](2026-06-07-security-low-priority.md)._

> **Remediation status (as of merge):** all **2 high + 7 medium fixed** — H-1/H-2/M-3 in PR #27, M-1/M-2/M-4 in #28, M-5/M-6/M-7 in #29; dependency refresh in #30. The 22 low findings remain a deferred backlog in the low-priority report.

## Result

| Severity | Count |
|---|---|
| 🔴 Critical | 0 |
| 🟠 High | 2 |
| 🟡 Medium | 7 |
| ⚪ Low (separate file) | 22 |

**No critical findings, no known-vulnerable dependencies** (`dotnet list package --vulnerable --include-transitive` is clean). Both high findings are the **voice-recording consent/authorization gap** (one issue, surfaced from two angles). The mediums are dominated by **upload/recording resource-exhaustion** and **two output-trust gaps** (message-content logging, unrestricted bot mentions).

The deployment model bounds blast radius: the bot serves **one configured guild**, so most "attacker" scenarios require an authorized-but-malicious guild member, not an anonymous internet user. The public surface (`/u/{id}`) is small and was already hardened (CSPRNG links, parameterized SQL, attachment disposition, nosniff — all re-verified).

---

## 🟠 HIGH

### H-1 · Any guild member can covertly record everyone in a voice channel
**CWE-862 / CWE-359** · `Services/VoiceRecorder.cs:67-101,163-171`; `Bot.cs:137-165` (registration), `Bot.cs:295-302` (routing)

`/recording` has **no permission gate** — not at registration (no `WithDefaultMemberPermissions`) and not in the handler. `action=="start"` only checks the caller is in a voice channel, then records **every non-bot user** in that channel (and anyone who joins later via `OnStreamCreated`). The only notice is an **ephemeral** "Joining voice channel and starting recording…" visible to the invoker alone — recorded participants get no announcement, DM, or opt-in. `FUTURE_FEATURES.md:47-48` already lists the consent system as unbuilt.

**Exploit:** a low-privilege member joins an active voice channel, runs `/recording start` (reply invisible to others), waits, then `/recording stop sendfile:true` — every participant's isolated mic track is captured to WAV and can be exfiltrated, with zero consent.

**Fix:**
1. Register `/recording` with `.WithDefaultMemberPermissions(GuildPermission.MoveMembers)` (or a configured operator role) **and** re-check `GuildPermissions` server-side in `HandleRecordingCommand` (DefaultMemberPermissions is admin-overridable, so it's not sufficient alone).
2. Post a **non-ephemeral, in-channel** announcement on recording start *and* stop.
3. Document covert-recording/consent as an explicit operator legal responsibility in the README.

> Verifier note: rated **high**, not critical — exploitation requires guild membership and the harm is privacy/surveillance within one community, not RCE or credential theft. But it is a real, unprivileged, no-consent capability available today, hence high.

### H-2 · `/recording stop sendfile:true` broadcasts each person's isolated mic to the whole text channel
**CWE-200** · `Services/VoiceRecorder.cs:489-492`; option at `Bot.cs:149-154`

The same authorization gap, distinct exposure: with `sendfile:true`, each **per-user** WAV is `SendFileAsync`'d into the text channel where `/recording stop` ran — an audience potentially much larger than the voice participants. Per-user isolation makes it worse than a mixed recording: any single voice is cleanly extractable and attributable.

**Fix:** restrict `sendfile` to privileged roles; prefer delivering recordings via ephemeral DM to the invoker or the existing unguessable `/upload` link rather than broadcasting; at minimum target a fixed private channel, never the invocation channel. (Folds into the H-1 permission gate.)

---

## 🟡 MEDIUM

### M-1 · Every Discord message body logged verbatim to stdout / container logs
**CWE-532** · `Bot.cs:92` (and `Bot.cs:391` for edits)

`OnMessageReceived` logs full `message.Content` of every non-bot message (the bot holds the privileged `MessageContent` intent). It lands in the 1000-line terminal buffer **and** `Console.Out` → captured by `podman logs` / the agent-cloud log pipeline, where it persists in central log retention beyond Discord's controls. No scrubbing or level gating. Anyone with log-store read access (or a compromised log-shipping credential) reads all guild conversation — a quiet continuous exfiltration channel.

**Fix:** don't log raw content at Info — log channel/author/length only, or gate behind a debug flag off in production. Apply to `OnMessageUpdated` too.

### M-2 · Bot echoes Ollama output with no `AllowedMentions` → `@everyone`/role-mention injection
**CWE-74** · `Services/WisLlmService.cs:265-271,116-118`

Model output is returned via `FollowupAsync(msg)` with **no `allowedMentions`** — and `AllowedMentions` is set nowhere in the codebase, so Discord's legacy default parses **all** mention tokens in content. The `/wisllm` prompt is attacker-controlled; the model can be told to emit literal `@everyone`, `<@&roleid>`, or `<@userid>`. Output is also persisted to `wisllm_history` and replayed into future context, so one poisoned answer can re-trigger.

**Exploit:** `/wisllm ask prompt:"Reply with exactly: @everyone free nitro <link>"` → if the bot role has Mention Everyone (common for utility bots), an on-demand mass-ping/social-engineering primitive; even without it, role/user pings still fire.

**Fix:** pass `allowedMentions: AllowedMentions.None` on every bot message carrying model output (and as a process-wide safe default). Where a legitimate ping is wanted (reminder fallback, M-4 below), build an explicit `AllowedMentions` with only the intended user id.

### M-3 · Unbounded in-memory audio buffering during recording (~11 MB/min/user)
**CWE-789** · `Services/VoiceRecorder.cs:30,239,340-365`

Every 20ms frame is a separate `AudioChunk`(`byte[]`) appended to an in-RAM `List` for the **entire session** — 48kHz/16-bit/stereo = ~11.25 MB/min/user, plus per-chunk object overhead. No duration or byte cap. `ReconstructAudio` then allocates a second contiguous per-user buffer at save (~2× peak).

**Exploit:** start recording in a busy channel and leave it: 10 speakers × 2h ≈ **13.5 GB** PCM in RAM → OOM-kills the container (taking `/health`, upload relay, and the gateway with it). One malicious member suffices.

**Fix:** cap recording duration and/or total buffered bytes (auto-stop + flush on exceed); better, stream PCM to a temp WAV on disk during capture so memory is O(frame) not O(session). (Pairs with the H-1 permission gate, which also limits who can trigger it.)

### M-4 · Reminder channel-fallback reflects user text beside a live mention with no `AllowedMentions`
**CWE-74** · `Services/ReminderService.cs:77`

`ch.SendMessageAsync($"⏰ <@{userId}> **Reminder:** {message}")` — the user-controlled reminder text is posted publicly alongside a real mention with no `AllowedMentions`. A reminder body containing `@everyone`/role tokens is parsed by Discord. Same root cause as M-2.

**Fix:** explicit `AllowedMentions` allowing only the target user id (`AllowEveryone=false`, no `RoleIds`).

### M-5 · No per-user cap on `/upload` → storage exhaustion
**CWE-770** · `Services/UploadService.cs:114-132`; `Config.cs:34-35`

`HandleUploadCommand` does no quota/rate check. Each call mints a link accepting one file up to **500 MB**, retained **30 days**; the sweep only deletes after expiry. A member scripting `/upload` accumulates up to 30 days of unbounded N×500MB objects, exhausting the MinIO volume for the whole guild.

**Fix:** per-owner cap on outstanding links and total stored bytes (`COUNT`/`SUM` from `uploads WHERE owner_user_id=? AND status!='expired'` before minting) + a short per-user `/upload` rate limit; reject over-quota with an ephemeral error. Consider lowering default retention.

### M-6 · Public upload buffers up to 500 MB to disk before storage; no rate/concurrency limit
**CWE-400** · `Services/WebService.cs:103-117`; `Config.cs:34`

`POST /u/{id}` calls `ReadFormAsync()` (fully materializing the multipart body — spilling >64KB to a temp file) **before** streaming to MinIO. Both `MaxRequestBodySize` and `MultipartBodyLengthLimit` = 500 MB, with **no rate limiting and no concurrency cap** on this public endpoint.

**Exploit:** mint many links (no cooldown), fire many concurrent 500 MB POSTs through the public Caddy subdomain; each buffers ~500 MB to container temp disk before MinIO → ephemeral-disk/memory exhaustion crashes the co-hosted bot.

**Fix:** add ASP.NET Core rate limiting (fixed-window/concurrency limiter keyed on `X-Forwarded-For` from Caddy) on `POST /u/{id}`; lower the default `UploadMaxBytes`; document a Caddy-level body/rate cap as the outer guard.

### M-7 · `POST /u/{id}` buffers the whole body *before* claiming the single-use link
**CWE-770** · `Services/WebService.cs:110-115`; `Services/UploadService.cs:165-177`

Closely related to M-6 but a specific, cheap fix: `ClaimForUploadAsync` (the single-use `pending→uploading` CAS) runs **inside** `StoreAsync`, *after* `ReadFormAsync` has already buffered the full body. So even requests that will lose the claim race still buffer ~500 MB first — one valid link permits unlimited concurrent buffered uploads.

**Fix:** check status and **claim the link before reading the body** (move the claim ahead of `ReadFormAsync`) so non-claimable requests are rejected without buffering; ideally stream the multipart section directly into MinIO with bounded buffering instead of materializing `IFormFile`.

---

## Dependencies

`dotnet list package --vulnerable --include-transitive` → **no vulnerable packages.** No security-driven upgrade is *required*. Staleness (`--outdated`), to refresh opportunistically alongside these fixes:

| Package | Current | Latest | Note |
|---|---|---|---|
| Discord.Net | 3.19.0-beta.1 | 3.20.0 | a **beta pinned in production**; a stable 3.20.0 exists — verify it still exposes the voice stream APIs before bumping |
| libsodium | 1.0.20 | 1.0.22 | voice-encryption native; low risk |
| Microsoft.Data.Sqlite | 10.0.5 | 10.0.8 | patch bumps |
| NAudio | 2.2.1 | 2.3.0 | minor |
| Concentus.Oggfile | 1.0.6 | 1.0.7 | **unused** — see low-priority report; remove rather than bump |

> Verifier note: the beta-in-production and unpinned-apt concerns were **down-rated to low/deferred** — the beta is the documented requirement for `IAudioClient.GetStreams()`, and apt pinning is marginal for a rebuilt-on-merge image. Details in the low-priority report.

---

## Suggested remediation order (start immediately, highest first)

1. **H-1 + H-2 + M-3 together** — one PR: permission-gate `/recording` (registration + server-side recheck), in-channel start/stop announcement, restrict/redirect `sendfile`, and add a recording duration/byte cap. This single PR closes both highs and the worst DoS.
2. **M-2 + M-4** — one small PR: a process-wide safe `AllowedMentions` default + explicit targeted mentions where pings are intended.
3. **M-1** — drop/redact raw message-content logging.
4. **M-5 + M-6 + M-7** — upload hardening PR: claim-before-buffer, per-user quota + rate limit, lower default cap.
5. **Dependency refresh** — bump the patch/minor packages; evaluate Discord.Net 3.20.0 stable separately (needs a voice smoke test).

All via the standard branch → PR → CodeRabbit → merge flow, `dotnet build` between pieces.
