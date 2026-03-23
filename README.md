# WisBot

A Discord bot that records voice channel audio and saves it as WAV files. Built with C# .NET 10 and Discord.Net.

## Requirements

- [.NET 10 SDK](https://dotnet.microsoft.com/download)
- A Discord bot token
- The bot must have the following permissions: **Connect**, **Speak**, **Read Messages/View Channels**, **Send Messages**, **Attach Files**
- Gateway intents required: **Server Members**, **Message Content**

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

## Output

WAV files are saved to `./recordings/` using the pattern `{username}_{timestamp}.wav`.
Audio is captured per-user at 48kHz, 16-bit stereo. Gaps in speech are filled with silence to maintain synchronization across users.

## Deployment

Production runs on a self-hosted runner via GitHub Actions. Trigger manually with `workflow_dispatch` in `.github/workflows/deployment_prod.yml`. The bot token is supplied via the `DISCORD_TOKEN_WISBOT` repository secret.
