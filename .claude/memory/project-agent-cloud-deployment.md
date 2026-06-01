---
name: project-agent-cloud-deployment
description: How WisBot is meant to deploy via the agent-cloud platform monorepo
metadata:
  node_type: memory
  type: project
---

WisBot deploys through the **agent-cloud** monorepo (`uhstray-io/agent-cloud`, public, Ansible/Semaphore/OpenBao platform). WisBot stays a **separate public repo** (own .NET toolchain) and integrates as an **AI/Agent-tier** service via the prebuilt-image pattern:

- WisBot **publishes a Docker image** (planned: `ghcr.io/uhstray-io/wisbot`); agent-cloud **pulls** it — no build-on-VM.
- Config arrives via an Ansible-templated `.env` (OpenBao secrets + site-config values). Container only *reads* env.
- Discord token lives in OpenBao `secret/services/discord` — never committed. Real guild IDs / IPs live in the private **site-config** repo (`/Users/stray/Documents/GitHub/site-config/`), never in this repo.
- Deploys run **only via Semaphore** (`deploy-wisbot.yml`); health verified over HTTP.

**Why:** agent-cloud is the org's single source of truth for deployments; public repos must contain zero secrets/IPs/real-IDs.

**How to apply:** Make site-specific values env-configurable in `Config.cs` (don't hardcode in `Bot.cs`); keep secrets out of git. Known blocker: voice uses Windows-only native libs (`OpusDotNet.opus.win-x64`) — needs Linux opus/libsodium for the container. Full plan: `docs/plans/2026-06-01-agent-cloud-deployment-alignment.md`. Related: [[feedback-no-coauthors]].
