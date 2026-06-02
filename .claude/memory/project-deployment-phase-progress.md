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

**Remaining:**
- Phase 7 — site-config inventory entry + seed `secret/services/wisbot` in OpenBao + go-live. **BLOCKED on site-config access + the real guild ID / WisAI Ollama endpoint.**
- Phase 8 — **File relay service** (new, planned): `/upload` → unguessable link → web upload (≤500MB) → same link downloads; bypasses Discord's ~8MB limit; 30-day retention. Decisions: **MinIO + DB metadata** for storage, **built into WisBot via ASP.NET Core/Kestrel** (Docker base `runtime`→`aspnet`), **trust-the-link** access, **public Caddy route + subdomain**. See the plan doc's Phase 8 section.

**How to apply:** Phases 5–6 live in the local agent-cloud clone; build Phase-6-style work in an isolated git worktree to avoid colliding with concurrent work, and stage specific files (never `git add -A`). Watch the shared CodeRabbit rate limit — space PRs out. No runtime OpenBao AppRole needed for WisBot (deploy-time token only). The bot's `/health` is internal (no Caddy), but Phase 8's upload site needs a public Caddy route.
