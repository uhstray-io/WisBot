# WisBot

A Discord bot with voice channel recording, welcome messages, and reminders. Built with C# .NET 10 and Discord.Net.

## Requirements

- [.NET 10 SDK](https://dotnet.microsoft.com/download)
- A Discord bot token
- The bot must have the following permissions: **Connect**, **Speak**, **Read Messages/View Channels**, **Send Messages**, **Attach Files**
- Gateway intents required: **Server Members Intent**, **Message Content Intent** — both must be enabled under your bot's settings in the [Discord Developer Portal](https://discord.com/developers/applications)

## Setup

1. Create a `discord.key` file at the repo root containing your bot token (one line, no quotes).
2. In `Bot.cs`, update `uhstrayGuildId` to your Discord server's guild ID.

```bash
dotnet restore
dotnet run
```

## Discord Commands

| Command | Description |
|---|---|
| `/recording start` | Join your current voice channel and begin recording all users |
| `/recording stop [sendfile] [mergeaudio]` | Stop recording and save WAV files; optionally send files to Discord or merge all users into one file |
| `/remind when:<duration> message:<text>` | Set a one-shot reminder; bot DMs you when time is up (e.g. `30m`, `2h`, `1d`, `1h30m`) |
| `/notify user:<@someone>` | DM you once the next time that user joins a voice channel; includes a link to join them |
| `/wisllm <prompt>` | Placeholder for future LLM integration |

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

- **Recordings** — WAV files saved to `./recordings/` as `{username}_{timestamp}.wav`. Captured per-user at 48kHz, 16-bit stereo; gaps in speech are filled with silence to keep users time-synchronized.
- **Database** — `wisbot.db` (SQLite) is created automatically at the app root on first run. Stores welcomed users and pending reminders; survives restarts.

## Deployment

WisBot ships as a Docker image. On merge to `main` (and on `v*` tags), `.github/workflows/build-and-publish.yml` builds the image and publishes it to `ghcr.io/uhstray-io/wisbot`. `.github/workflows/docker-build.yml` validates the image builds on every PR.

The image is deployed by the **agent-cloud** platform — it pulls the published image and supplies configuration (Discord token, guild ID, endpoints) via an Ansible-templated `.env` from OpenBao + site-config, orchestrated through Semaphore. Nothing site-specific is baked into the image. See `docs/plans/2026-06-01-agent-cloud-deployment-alignment.md`.

> The legacy self-hosted-runner deploy workflows have been removed in favor of this model. (`deploy-o11y.yml` remains for now; migrating observability to agent-cloud's o11y service is a tracked follow-up.)
