# WisBot Kickstart

The starting point for anyone working in this project. This page gets you from a fresh
clone to a running bot, orients you in the codebase, and explains how a change travels
from your machine to production. Deeper detail lives in the linked docs — this is the map.

## What WisBot Is

A Discord bot in **C# / .NET 10** (Discord.Net) with:

- **Voice recording** — `/recording start|stop` captures per-user audio in a voice channel
  and writes time-synchronized WAV files (optionally merged into one).
- **File relay** — `/upload` mints an unguessable web link; anyone with the link uploads a
  file (≤500 MB by default, stored in MinIO) and the same link downloads it. Bypasses
  Discord's attachment limit; 30-day retention.
- **Reminders** — `/remind` one-shot reminders delivered by DM.
- **Voice notifications** — `/notify` DMs you the next time a user joins a voice channel.
- **Welcomes, voice stats, status** — first-join welcome messages, `/voicestats`, `/status`.
- **WisLLM** — `/wisllm ask|clear|compact` chats against an Ollama endpoint.

It is a single console app: a Discord client + an embedded Kestrel web server
(`/health`, upload/download routes) + SQLite for persistence + MinIO for file bytes.

## Repo Map

```text
Program.cs            Entry point — wires Terminal + Bot, keeps the process alive
Bot.cs                Discord client lifecycle, slash command registration & routing
Terminal.cs           Async console I/O + terminal commands (/testrecord, /gc, ...)
Database.cs           SQLite schema — all tables created here on startup
Config.cs             All settings: env var → .env file → default
Services/             One file per feature (VoiceRecorder, UploadService, WebService, ...)
compose.yaml          LOCAL compose — builds this checkout + MinIO (Podman or Docker)
Dockerfile            Multi-stage Linux image (aspnet base) — what CI publishes
.env.example          Every config knob, documented
.github/workflows/    docker-build (PR validation), build-and-publish (GHCR on merge)
architecture/         Voice recording internals, storage design, roadmap
docs/plans/           Deployment alignment plan, local-dev follow-up notes
.claude/memory/       Shared project knowledge base (decisions, conventions)
CLAUDE.md             Architecture reference + coding conventions (read this second)
```

## Prerequisites

1. **[.NET 10 SDK](https://dotnet.microsoft.com/download)** — the only hard requirement
   to build and run.
2. **A Discord application** ([Developer Portal](https://discord.com/developers/applications)):
   - A **bot token**.
   - Gateway intents enabled: **Server Members Intent** and **Message Content Intent**.
   - Invited to your server with: Connect, Speak, View Channels, Send Messages, Attach Files.
   - Your **guild (server) ID** — right-click your server → *Copy Server ID* (enable
     Developer Mode in Discord settings if you don't see it).
3. **Optional, per feature:**
   - **Podman** (or Docker) — only needed for the file relay's MinIO. Podman is the
     project's preferred local runtime (open source, daemonless).
   - **libopus** — only needed for voice *recording* (Windows: bundled via NuGet;
     macOS: `brew install opus`; Linux: `apt-get install libopus0`). Everything else
     runs without it.
   - **An Ollama endpoint** — only for `/wisllm`.

## First Run (5 minutes)

```bash
git clone https://github.com/uhstray-io/WisBot.git && cd WisBot
cp .env.example .env
# edit .env: set DISCORD_TOKEN_WISBOT and WISBOT_GUILD_ID (both required)
dotnet run
```

Verify: the console shows the bot connecting, slash commands register in your server, and
`curl http://localhost:8080/health` returns `200` once Discord is connected (`503` while
starting). The SQLite database (`wisbot.db`) is created automatically on first run.

> The token can alternatively live in a `discord.key` file at the repo root (one line, no
> quotes). Both `.env` and `discord.key` are gitignored — **never commit secrets.**

## Development Loops

Config always resolves **environment variable → `.env` → default** (see `.env.example`
for every knob). Pick the loop that fits what you're touching:

| Loop | Command | Works on |
|---|---|---|
| Validate the build | `dotnet build` | any OS, identically |
| Run natively (default loop) | `dotnet run` | any OS (voice needs libopus) |
| File relay (`/upload`) locally | `podman compose up -d minio` + `dotnet run` | any OS |
| Full stack in containers | `podman compose up --build` | Linux / Windows / Intel Mac |

**Native run is the universal loop** — the Windows-only opus NuGet package is
conditioned out of the csproj on other OSes, so `dotnet build` / `dotnet run` behave the
same everywhere.

### File relay locally

The relay needs MinIO. On macOS/Windows, create the Podman VM once
(`podman machine init && podman machine start`; Linux skips this), then:

```bash
podman compose up -d minio        # MinIO only — multi-arch, runs native
dotnet run
```

with these in `.env`:

```ini
WISBOT_MINIO_ENDPOINT=localhost:9000
WISBOT_MINIO_ACCESS_KEY=minioadmin
WISBOT_MINIO_SECRET_KEY=minioadmin
```

MinIO console: `http://localhost:9001` (minioadmin / minioadmin). Upload links resolve at
`http://localhost:8080/u/...`. Note: the `uploads` table is created on the bot's `OnReady`,
so the relay needs a real Discord login — a dummy token won't get there.

### Full stack in containers

`compose.yaml` at the root builds your checkout and runs WisBot + MinIO together
(`podman compose up --build`, or the same files under `docker compose`).

> **⚠️ Apple Silicon:** the all-in-container *build* does not work — the arm64 .NET 10
> SDK SIGILLs in the Podman VM, and the `linux/amd64` image fails under qemu emulation.
> This is environment-specific, not a code issue (CI builds the same image fine on native
> amd64). Use the native-bot + MinIO-container loop above. Details:
> `docs/plans/2026-06-04-local-dev-followup.md`.
>
> This local compose is **separate from deployment** — deployment runs the *published*
> image with its own config (next section).

## How Deployment Works

WisBot ships as an OCI image; nothing site-specific is ever baked in. A change reaches
production like this:

```text
your branch ──PR──▶ main
   │                 │
   │  docker-build.yml          build-and-publish.yml
   │  (validates image          (builds + pushes image)
   │   builds, every PR)               │
   │                                   ▼
   │                    ghcr.io/uhstray-io/wisbot
   │                                   │
   └──────────── agent-cloud platform pulls the image ◀──── Semaphore runs
                                       │                    deploy-wisbot.yml
                          Ansible templates .env from:
                          • OpenBao  → secrets (Discord token)
                          • site-config → real IDs, endpoints, IPs
                                       │
                                       ▼
                          container runs; /health verified over HTTP
```

**Division of responsibility:**

| Concern | Lives in |
|---|---|
| Code, Dockerfile, CI that publishes the image | **this repo** (public — zero secrets/IDs) |
| Deploy playbooks, compose, env template (`agents/wisbot/`) | **agent-cloud** repo (public) |
| Secrets (Discord token → `secret/services/wisbot`) | **OpenBao** |
| Real guild IDs, IPs, endpoints | **site-config** repo (private) |
| Deploy execution | **Semaphore** (only path to deploy — no manual container runs) |

**What a deployment needs:** the published image (automatic on merge), the OpenBao secret
seeded, the site-config inventory entry, and a Semaphore run of `deploy-wisbot.yml`. The
bot's `/health` is internal; the file relay's upload site additionally needs a public
Caddy route/subdomain.

**Current status:** the repo-side and agent-cloud-side work is merged (image publishes on
every merge to `main`; deploy playbooks exist). Go-live is pending infra values and
operator actions (OpenBao seed, site-config entry, Semaphore provision). Full plan and
status: `docs/plans/2026-06-01-agent-cloud-deployment-alignment.md` and
`.claude/memory/project-deployment-phase-progress.md`.

## Contributing Workflow

- **All changes go through a PR** to `main` — never push directly. Merge only when
  **CodeRabbit** fully passes: resolve its findings, re-request review, then merge.
- Validate with `dotnet build` before every PR (no test framework is configured yet).
- Follow the conventions in `CLAUDE.md` — modern C# 14 style (file-scoped namespaces,
  collection expressions, primary constructors), manual constructor DI, one service per
  file under `Services/`, all config via `Config.Load()` (never hardcode site values).
- Stage specific files (`git add <file>`), never `git add -A`.
- Project decisions and their rationale live in `.claude/memory/` — check it before
  re-litigating a decision, add to it when you make one.

## Where to Go Deeper

| Topic | Doc |
|---|---|
| Architecture, conventions, every service explained | `CLAUDE.md` |
| Voice recording internals & troubleshooting | `architecture/VOICE_RECORDING_README.md` |
| Storage design decisions | `architecture/AUDIO_STORAGE_ARCHITECTURES.md` |
| Roadmap / planned features | `architecture/FUTURE_FEATURES.md` |
| Deployment plan (phases, decisions) | `docs/plans/2026-06-01-agent-cloud-deployment-alignment.md` |
| Local-dev status & Apple Silicon details | `docs/plans/2026-06-04-local-dev-followup.md` |
| Every config option | `.env.example` |
