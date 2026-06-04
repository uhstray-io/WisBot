# Local Development — Follow-up / Pick-up Notes

_Last updated: 2026-06-04_

Context for resuming the "run WisBot locally on any OS" work. Goal: build the C#
binary and run the app on a developer machine (any OS) **without** pushing to the
monorepo or redeploying on hardware. Runtime preference: **Podman** (open source,
daemonless) over Docker — for **local dev only**; deployment was intentionally left
on the existing model.

## Status: DONE (merged) for local dev

Two PRs merged to `main`:

- **#19** — `feat: OS-agnostic local development (compose + MinIO, docs, .env ignore)`
- **#20** — `feat: Podman-first local dev; honest Apple Silicon container caveat`

What exists now:

- `compose.yaml` (root, tool-neutral name) — builds this checkout + runs WisBot + MinIO.
  Works under both `podman compose` and `docker compose`. MinIO is multi-arch (native);
  the `wisbot` service pins `platform: linux/amd64` (image bundles the x86_64 `libopus`).
- `.gitignore` ignores `.env` / `.env.local` (the bot token can live in `.env`).
- README "Local Development" + a note in `CLAUDE.md` — Podman-first, per-OS guidance.

## Local development loops

Token for the running bot comes from either `DISCORD_TOKEN_WISBOT` (in `.env` or the
environment) or a `discord.key` file at the repo root — both are supported, equally.

| Loop | Command | Works on |
|---|---|---|
| Validate build | `dotnet build` | macOS / Windows / Linux (native) |
| Run natively | `dotnet run` (token via `.env` or `discord.key`) | any OS (voice needs `libopus`*) |
| Full stack in container | `podman compose up --build` | Linux / Windows / **Intel** Mac |
| File relay on Apple Silicon | `podman compose up -d minio` + `dotnet run` | Apple Silicon (native bot + MinIO container) |

*Voice recording needs `libopus` at runtime: NuGet on Windows, `brew install opus`
on macOS, `libopus0` on Linux. Everything else runs without it (opus only loads on
voice-join).

## ⚠️ Apple Silicon caveat (verified 2026-06-04, Podman 5.8.2, applehv VM)

The **all-in-container build does NOT work on Apple Silicon.** Two distinct failures:

- **Native arm64** in the podman VM: .NET 10.0.300 SDK → `Illegal instruction` (SIGILL).
- **Emulated amd64** (qemu, the `platform: linux/amd64` pin): `dotnet restore` →
  `MSB4184: GetTargetFrameworkVersion(net6.0) … exception thrown by the target of an
  invocation`.

This is **environment-specific, not a code issue** — proof: the native-amd64 CI
(`build-and-publish.yml`) built and published the image successfully on the #19 merge.
So **deployment is unaffected.** On Apple Silicon, use the native-bot + MinIO-container
loop (last table row).

## Verified vs. not verified

Verified on this Apple Silicon Mac:
- `dotnet build` clean; `dotnet run` boots and serves `/health` → `503` (Kestrel up,
  Discord not connected with a dummy token — expected).
- MinIO in a Podman container: health `200`, console `200` (native arm64).
- `podman compose config` validates the file under the real provider.

NOT verified:
- The full `/upload` round-trip through MinIO. It needs a real Discord login →
  `OnReady` → `Database.Initialize()` to create the `uploads` table; a dummy token
  can't reach that. **To finish verification:** put a real token in `.env`
  (`DISCORD_TOKEN_WISBOT`) + a real `WISBOT_GUILD_ID`, start MinIO, `dotnet run`, then
  `/upload` in Discord and exercise the returned link.

## Quick resume — Apple Silicon commands

```bash
# one-time: podman machine (already created on this Mac during the 2026-06-04 session)
podman machine start                      # `podman machine list` to check; `stop` to pause

# MinIO only (native), then bot natively
podman compose up -d minio
# .env: DISCORD_TOKEN_WISBOT=<real>, WISBOT_GUILD_ID=<real>,
#       WISBOT_MINIO_ENDPOINT=localhost:9000, WISBOT_MINIO_ACCESS_KEY=minioadmin,
#       WISBOT_MINIO_SECRET_KEY=minioadmin
dotnet run

# health + MinIO console
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/health   # 503 until Discord-connected
open http://localhost:9001                                              # minioadmin / minioadmin
```

This Mac now has a `podman machine` VM + cached .NET SDK images (~a couple GB).
Reclaim with `podman machine rm && podman system prune -a` if needed.

## OPEN DECISION (deferred — pick this up)

**Should agent-cloud's deployment be retargeted from Docker to Podman?** Left on Docker
for now. The published image is a standard OCI image that runs identically under either
runtime, so the lean is: **leave deployment as-is** — little to gain, real risk in
swapping the platform's container engine just for WisBot. Revisit only if the org
standardizes on Podman platform-wide. Deployment alignment plan:
`docs/plans/2026-06-01-agent-cloud-deployment-alignment.md`.

## Unrelated, still pending (not part of this work)

- Phase 7 + 8c go-live (deploy): needs infra values (VM IP/VMID/node) + operator
  actions (OpenBao seed, provision, Semaphore). Tracked in site-config `plan/NEXT-STEPS.md`.
- `docs/superpowers/plans/2026-04-16-structural-improvements.md` — a separate, older,
  largely-unaddressed refactor plan (Features/ folders, namespaces, config consolidation,
  log levels, command registry). Appears stale vs. the current `Services/` layout; needs
  a decision (close as superseded, or re-scope).
