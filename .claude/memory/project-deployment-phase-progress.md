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

**Remaining — agent-cloud + site-config repos:**
- Phase 5 — `agents/wisbot/` deploy dir (compose pulls the image, deploy.sh container-only, `wisbot.env.j2`) + `platform/tests/test_service_wisbot.bats`. *(agent-cloud, AI-agent tier — note: agents live under `agents/`, NOT `platform/services/`.)*
- Phase 6 — composable `deploy-wisbot.yml` (NetBox-style: sparse-checkout → manage-secrets → run-deploy → verify-health) + Semaphore template.
- Phase 7 — site-config inventory entry + seed `secret/services/wisbot` in OpenBao + go-live. **BLOCKED on site-config access + the real guild ID / WisAI Ollama endpoint.**

**How to apply:** Pick up at Phase 5 in the agent-cloud repo (`/Users/jacobhaig/Documents/GitHub/agent-cloud`). Watch the shared CodeRabbit rate limit — space PRs out. No runtime OpenBao AppRole needed for WisBot (deploy-time token only). WisBot needs no Caddy route (internal `/health`).
