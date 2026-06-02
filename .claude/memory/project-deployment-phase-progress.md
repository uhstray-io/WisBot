---
name: project-deployment-phase-progress
description: Status of the WisBot → agent-cloud deployment alignment (7-phase plan)
metadata:
  node_type: memory
  type: project
---

Tracking the phased migration in [[project-agent-cloud-deployment]]. Full plan: `docs/plans/2026-06-01-agent-cloud-deployment-alignment.md`. Workflow per phase = one branch → PR → CodeRabbit → merge (see [[feedback-coderabbit-pr-workflow]]).

**Milestone A — WisBot repo: COMPLETE (merged to main as of 2026-06-01)**
- Phase 1 (#9) — config externalized to env vars (`WISBOT_GUILD_ID`, DB/recordings paths, etc.); `Config.Load` reads env → `.env` → default.
- Phase 2 (#10) — HTTP `/health` endpoint (`HttpListener`, `WISBOT_HEALTH_HOST`/`PORT`).
- Phase 3 (#11) — multi-stage Dockerfile; Linux voice via apt `libopus0` (libsodium/SQLite natives come from NuGet); `docker-build.yml` validates build on PRs.
- Phase 4 (#12) — `build-and-publish.yml` pushes to `ghcr.io/uhstray-io/wisbot` on main/tags; legacy self-hosted deploy workflows removed (`deploy-o11y.yml` kept). **First GHCR publish succeeded.**

**Milestone B — agent-cloud repo:**
- Phase 5 (agent-cloud #43) — **MERGED.** `agents/wisbot/` deploy dir (compose pulls the image, deploy.sh container-only, `wisbot.env.j2`) + `platform/tests/test_service_wisbot.bats`. *(AI-agent tier — agents live under `agents/`, NOT `platform/services/`.)*
- Phase 6 (agent-cloud #45) — **MERGED.** `deploy-wisbot.yml` (clone + manage-secrets → deploy.sh → verify-health), `clean-deploy-wisbot.yml`, Semaphore templates, `validate-all` block. Added a backward-compatible `_templates_src` override to `manage-secrets.yml` so `agents/` services can template env files. *(The composable task files in the AUTOMATION-COMPOSABILITY doc — sparse-checkout/run-deploy/verify-health — don't exist yet; real playbooks inline git+shell+uri, which deploy-wisbot mirrors.)*

**Milestone D — file relay (`/upload`): code-complete (merged to WisBot main):**
- Phase 8a-1 (#14) — health server migrated to ASP.NET Core/Kestrel (Docker base `runtime`→`aspnet`).
- Phase 8a-2 (#15) — `/upload` command + `uploads` DB table + unguessable-link minting.
- Phase 8a-3 (#16) — MinIO 7.0 storage + streamed `GET/POST /u/{id}` + `/u/{id}/file` (one-file-per-link atomic claim, 413 cap, forced-attachment download); gated on `UploadEnabled`.
- Phase 8b (#17) — hourly retention loop deletes expired uploads (object + row).

**Remaining (need infra values + operator/Semaphore actions — I can't provision/seed/deploy):**
- Phase 7 (go-live) + Phase 8c (file-relay deploy) — make the site-config edits (VM, inventory `wisbot_svc`, `vm-specs`, Caddy `wisbot.uhstray.io`), seed `secret/services/wisbot` (token + MinIO creds), provision the VM, run Semaphore "Deploy WisBot". **Pending the VM IP/VMID/node (TBD).** Tracked in **site-config `plan/NEXT-STEPS.md`** (site-config PR #1).

**Decisions locked:** secrets under a new `secret/services/wisbot` (not reusing `discord`); upload subdomain **`wisbot.uhstray.io`**; guild ID comes from inventory `wisbot_guild_id`.

**How to apply:** WisBot-repo work = normal branch/PR. agent-cloud + site-config work = **isolated worktree off `main`** + stage specific files (never `git add -A`). For **site-config**, follow [[reference-site-config-repo]] (private; additive-only; never leak its contents). Per [[feedback-coderabbit-pr-workflow]], merge non-major resolved findings after fix + green CI.
