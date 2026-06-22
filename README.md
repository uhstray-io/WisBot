# WisBot

A Discord bot with voice channel recording, welcome messages, and reminders. Built with C# .NET 10 and Discord.Net.

> **New to the project?** Start with [KICKSTART.md](KICKSTART.md) — setup, development loops, and how deployment works, all in one place.

## Requirements

- [.NET 10 SDK](https://dotnet.microsoft.com/download)
- A Discord bot token
- The bot must have the following permissions: **Connect**, **Speak**, **Read Messages/View Channels**, **Send Messages**, **Attach Files**
- Gateway intents required: **Server Members Intent**, **Message Content Intent** — both must be enabled under your bot's settings in the [Discord Developer Portal](https://discord.com/developers/applications)

## Setup

1. Provide your bot token, either:
   - a `discord.key` file at the repo root (one line, no quotes), **or**
   - `DISCORD_TOKEN_WISBOT` in a `.env` file (copy `.env.example` → `.env`).
2. Set `WISBOT_GUILD_ID` (your Discord server's guild ID) in `.env` — it is **required**.

```bash
dotnet restore
dotnet run
```

## Local Development

The same lane works on **macOS, Windows, and Linux**. Config resolves in order:
environment variable → local `.env` → default (see `.env.example`).

### 1. Validate the build (any OS)

```bash
dotnet build
```

The Windows-only `opus` native package is conditioned out on macOS/Linux, so the build
succeeds identically on every OS.

### 2. Run natively

```bash
dotnet run        # reads discord.key / .env at the repo root
```

Voice **recording** additionally needs the `libopus` native at runtime:

| OS | How |
|---|---|
| Windows | bundled automatically via NuGet (`OpusDotNet.opus.win-x64`) |
| macOS | `brew install opus` |
| Linux | `apt-get install libopus0` |

The bot **starts and runs every non-voice feature without it** — opus only loads when you
join a voice channel — so you can develop most features on any machine without extra setup.

### 3. Run the file relay (`/upload`) locally

The relay needs a MinIO. **Podman** is the primary local runtime (open source, daemonless);
its commands below also work under Docker. On macOS/Windows, start the VM once:
`podman machine init && podman machine start` (Linux skips this).

**Apple Silicon Macs — native bot + MinIO container (recommended):**

Building the WisBot *image* on Apple Silicon currently fails (see the caveat below), so run
the bot natively and put only MinIO in a container:

```bash
podman compose up -d minio          # MinIO only (multi-arch, runs native)
# in .env: WISBOT_MINIO_ENDPOINT=localhost:9000  (+ access/secret = minioadmin)
dotnet run
```

**Linux / Windows / Intel Mac — full stack in one command:**

`compose.yaml` builds this checkout and brings up WisBot **and** MinIO together:

```bash
cp .env.example .env                 # set DISCORD_TOKEN_WISBOT + WISBOT_GUILD_ID
podman compose up --build            # or: docker compose up --build
```

- Health: `http://localhost:8080/health` · Upload links: `http://localhost:8080/u/...`
- MinIO console: `http://localhost:9001` (login `minioadmin` / `minioadmin`)

> **Apple Silicon caveat:** the all-in-container build is blocked by .NET 10.0.300 toolchain
> bugs in the Podman VM — the arm64 SDK hits an illegal instruction (SIGILL), and the
> `linux/amd64` image (pinned for the x86_64 `libopus`) fails under qemu emulation with an
> MSBuild error. This is environment-specific, **not** a code issue: the same image builds
> cleanly on native amd64 (CI publishes it on every merge). Use the native+MinIO path above.

> This is the **local** compose (builds your checkout). It is separate from the deployment
> path below, which runs a pulled image with its own config.

## Discord Commands

| Command | Description |
|---|---|
| `/recording start` | Join your current voice channel and begin recording all users |
| `/recording stop [sendfile] [mergeaudio]` | Stop recording and save WAV files; optionally send files to Discord or merge all users into one file |
| `/remind when:<duration> message:<text>` | Set a one-shot reminder; bot DMs you when time is up (e.g. `30m`, `2h`, `1d`, `1h30m`) |
| `/notify user:<@someone>` | DM you once the next time that user joins a voice channel; includes a link to join them |
| `/upload` | Mint a one-file upload link (file relay; only registered when MinIO is configured via `WISBOT_MINIO_ENDPOINT`) |
| `/status` | Monitoring snapshot of the bot process |
| `/voicestats [user]` | Per-user voice activity stats from the tracked join/leave history |
| `/wisllm ask prompt:<text> [model:<name>]` | Ask WisLLM (Ollama-backed) a question; requires `OLLAMA_ENDPOINT`. Sessions are shared per guild, per-user in DMs |
| `/wisllm clear` | Start a fresh WisLLM conversation session |
| `/wisllm compact` | Summarize the current WisLLM session into a single context and continue from it |

## Terminal Commands

While the bot is running, type these in the console:

| Command | Description |
|---|---|
| `/testrecord` | Join a hardcoded test channel, record 15 seconds, save, and leave |
| `/removeallcommands` | Delete all registered slash commands from Discord |
| `/gc` | Force a .NET garbage collection |
| `/clear` | Clear the terminal output |
| `?` or `/help` | Show available commands |

## Data

- **Recordings** — WAV files saved to `./recordings/` as `{username}_{timestamp}.wav`. Captured per-user at 48kHz, 16-bit stereo; gaps in speech are filled with silence to keep users time-synchronized. Auto-deleted after `WISBOT_RECORDINGS_RETENTION_DAYS` (default 30). Treat the recordings volume as sensitive.
- **Voice activity** — the bot **passively records every member's voice channel joins and leaves** (to power `/voicestats`). This is logged for all members continuously; rows auto-delete after `WISBOT_VOICE_ACTIVITY_RETENTION_DAYS` (default 90). Operators should disclose this to their guild.
- **WisLLM history** — conversation prompts/responses are stored in `wisbot.db` and auto-deleted after `WISLLM_HISTORY_RETENTION_DAYS` (default 30).
- **Database** — `wisbot.db` (SQLite) is created automatically at the app root on first run. Stores welcomed users, pending reminders, voice activity, WisLLM history, and upload metadata; survives restarts.

## Deployment

WisBot ships as a Docker image. On merge to `main` (and on `v*` tags), `.github/workflows/build-and-publish.yml` builds the image and publishes it to `ghcr.io/uhstray-io/wisbot`. `.github/workflows/docker-build.yml` validates the image builds on every PR.

The image is deployed by the **agent-cloud** platform — it pulls the published image and supplies configuration (Discord token, guild ID, endpoints) via an Ansible-templated `.env` from OpenBao + site-config, orchestrated through Semaphore. Nothing site-specific is baked into the image. See `docs/plans/2026-06-01-agent-cloud-deployment-alignment.md`.

> The legacy self-hosted-runner deploy workflows have been removed in favor of this model. (`deploy-o11y.yml` remains for now; migrating observability to agent-cloud's o11y service is a tracked follow-up.)
