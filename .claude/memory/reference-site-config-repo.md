---
name: reference-site-config-repo
description: How to use the private site-config repo (real IPs/creds) without ever leaking it
metadata:
  node_type: memory
  type: feedback
---

The org's private infrastructure config lives in the **`uhstray-io/site-config`** repo (PRIVATE). It is the ONLY place real IP addresses, credentials, production inventory, Caddy config, and VM specs live — agent-cloud, WisBot, and all public repos use placeholder variables that site-config values are injected into at runtime. OpenBao is the runtime source of truth; site-config's `secrets/` are file-based backups. (This memory deliberately records **no actual values** from it.)

**STRICT handling rules (user-directed — make these obvious):**
1. **Never copy site-config contents out.** Real IPs, IDs, hostnames, secrets, or topology must NEVER be written into WisBot, agent-cloud, any public repo, their commit messages / PR bodies, or any memory file — those stay placeholder-only. Do not record evidence of site-config's contents anywhere outside site-config itself.
2. **Additive & surgical only.** When editing site-config, only ADD/adjust the WisBot-specific config. Never remove or modify entries belonging to other services or infrastructure.
3. **Isolate.** The user actively works there on feature branches — always work in a separate branch/worktree off `main`; never edit their checked-out branch.
4. **Minimize echo.** Avoid printing real secret/IP values even in chat; reason in terms of keys/structure.

**Why:** site-config is private org infrastructure the user explicitly entrusted access to on these terms. Leaking its contents (even into a committed memory file) or disturbing non-WisBot config is a serious breach.

**How to apply:** read site-config to do WisBot work (inventory, vm-specs, Caddy, OpenBao secret layout), but keep every other repo and memory **placeholder-only**. Pending WisBot infra edits + the values still to decide are tracked inside site-config's own `plan/NEXT-STEPS.md`. Related: [[project-agent-cloud-deployment]], [[project-deployment-phase-progress]], [[feedback-coderabbit-pr-workflow]].
