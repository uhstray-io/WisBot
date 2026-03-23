# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
dotnet restore        # Restore NuGet packages
dotnet build          # Build the project
dotnet run            # Run the bot (requires discord.key file with bot token)
dotnet build -c Release  # Release build
```

No test framework or linter is configured.

## Deployment

Production runs via Docker on a self-hosted GitHub Actions runner (see `.github/workflows/deployment_prod.yml`). The Discord token is injected via a `DISCORD_TOKEN_WISBOT` secret appended to `.env` before `docker build`. Deployment is manually triggered (`workflow_dispatch`), not on push.

## Architecture

C# .NET 10.0 console application — a Discord bot with voice recording, welcome messages, and reminders.

### Source Files (all at repo root)

- **Program.cs** — Entry point. Creates `Terminal` and `Bot` with bidirectional references, runs both concurrently via fire-and-forget tasks, blocks with `Task.Delay(-1)`.
- **Bot.cs** — Discord client lifecycle, event subscriptions, slash command registration and execution. Manages `DiscordSocketClient` connection. Owns feature service instances and wires events.
- **Terminal.cs** — Async console I/O with a `ConcurrentQueue<string>` log buffer (max 1000 lines). Dispatches terminal commands (`/testrecord`, `/removeallcommands`, `/gc`, `/clear`) that call into Bot.
- **VoiceRecorder.cs** — Voice channel audio capture. Stores per-user audio as timestamped sparse `AudioChunk` objects in a `ConcurrentDictionary<ulong, UserAudio>`. Reconstructs sparse chunks into continuous PCM, writes WAV files via NAudio `WaveFileWriter`. Optionally merges multi-user recordings into a single file.
- **WelcomeHandler.cs** — Handles `UserJoined` events. Sends a randomized welcome message the first time a user joins a guild; tracks welcomed users in the `welcomed_users` DB table to suppress re-welcomes on rejoin.
- **ReminderService.cs** — One-shot reminder scheduler. Persists reminders to the `reminders` DB table. Background loop fires every 30s, claiming due reminders atomically via `DELETE ... RETURNING`. Delivers via DM with channel mention fallback. Also owns `TryParseDuration` / `FormatDuration` static helpers.
- **Database.cs** — Static helper. Owns the SQLite connection string (`wisbot.db`) and runs `CREATE TABLE IF NOT EXISTS` for all feature tables on startup.

### Startup Flow

```
Program.cs → new Terminal() + new Bot(terminal)
           → terminal.Bot = bot  (bidirectional link)
           → terminal.NewTerminal()  (async console loop)
           → bot.StartBot()          (Discord connection + event setup)
           → Task.Delay(-1)          (keep alive)
```

### Audio Recording Pipeline

1. `/recording start` → `JoinAndRecordChannel()` connects to voice channel via `IAudioClient`
2. `StreamCreated`/`StreamDestroyed` events track users joining/leaving mid-session
3. `ReadStream()` loop reads 3840-byte frames (20ms at 48kHz/16-bit/stereo), creates timestamped `AudioChunk` per frame; a 5-second read timeout detects broken streams and nulls them for re-subscription
4. `/recording stop` → cancels recording tasks (10s timeout), unsubscribes events, disconnects; calls `SaveAllUsersAsWav()` which reconstructs sparse chunks into continuous PCM (filling gaps with silence) and writes WAV files to `./recordings/`
5. Optional: `MergeAudioFiles()` sums all user WAVs into one file by mixing 16-bit samples with clamping

## Conventions

- **Manual constructor DI** — `Terminal` is injected into `Bot`, `VoiceRecorder`, `WelcomeHandler`, and `ReminderService` (no DI container); `DiscordSocketClient` is injected into services that need to call Discord APIs after startup
- **Feature file pattern** — Each feature lives in its own file (`WelcomeHandler.cs`, `ReminderService.cs`). `Bot.cs` owns instances, wires events, and handles slash command routing. `Database.cs` owns schema for all features.
- **Discord timeout pattern** — Long operations use `Task.Run()` fire-and-forget after immediate `RespondAsync()`, then `FollowupAsync()` for results (avoids 3-second Discord interaction timeout)
- **Thread safety** — `ConcurrentDictionary` and `ConcurrentQueue` throughout; background service lists use `lock` where needed
- **Audio constants** — 48kHz sample rate, 16-bit depth, 2 channels (stereo), 3840 bytes per 20ms frame
- **Config** — Discord token read from `discord.key` at repo root (gitignored). Guild/user IDs are hardcoded constants in `Bot.cs`
- **Recording output** — WAV files saved to `./recordings/` with pattern `{username}_{timestamp}.wav`
- **Command registration** — Slash commands are registered idempotently on `OnReady` (checks existing before creating); use `/removeallcommands` terminal command to force-clear all registered commands
- **Database** — SQLite via `Microsoft.Data.Sqlite`. Single file `wisbot.db` at app root. All tables declared in `Database.Initialize()`. Discord `ulong` IDs are stored as `long` (SQLite INTEGER) and cast at the boundary. ISO 8601 strings (`"O"` format) used for `DateTime` storage.
- **Atomic reminder claiming** — `DELETE ... RETURNING` used to atomically consume due reminders, preventing double-delivery on restart

## Key Dependencies

- **Discord.Net 3.19.0-beta.1** — Beta version required for voice audio stream features (`IAudioClient.GetStreams()`). `GuildMembers` privileged intent required for `UserJoined` events (must be enabled in Discord Developer Portal).
- **NAudio 2.2.1** — WAV file writing and audio format handling
- **Microsoft.Data.Sqlite 10.0.5** — Embedded SQLite database, no server required
- **OpusDotNet.opus.win-x64 + libsodium** — Native libraries for Discord voice codec and encryption
- **Concentus.Oggfile** — Included but not currently used

## Documentation

- `architecture/VOICE_RECORDING_README.md` — Voice recording implementation details and troubleshooting
- `architecture/FUTURE_FEATURES.md` — Planned features and roadmap
- `architecture/AUDIO_STORAGE_ARCHITECTURES.md` — Storage design decisions



# Coding Conventions for Claude

## C# 14 & .NET 10 Coding Conventions

You are an expert .NET 10 developer. Follow these "Modern & Simple" rules:

### 1. Modern C# 14 Syntax (Mandatory)
* **Field-backed Properties:** Use the `field` keyword for properties with logic to avoid manual backing fields.
    * *Bad:* `private string _name; public string Name { get => _name; set => _name = value; }`
    * *Good:* `public string Name { get; set => field = value?.Trim(); }`
* **Null-Conditional Assignment:** Use `?.=` for defensive assignments.
    * *Example:* `options?.LoggingLevel = LogLevel.Debug;`
* **Extension Blocks:** Use the new `extension` keyword for grouping extension members (properties, methods, and static members).
* **Primary Constructors:** Use primary constructors for classes and structs unless complex initialization logic is required.
* **Collection Expressions:** Always use `[]` for empty or populated collections/spans (e.g., `List<int> items = [1, 2, 3];`).

### 2. General Style & Clean Code
* **File-Scoped Namespaces:** Use `namespace MyProject.Models;` (no curly braces for the namespace).
* **Implicit Usings:** Rely on implicit usings. Do not include standard system imports unless unique.
* **Var Keyword:** Use `var` only when the type is obvious from the right side of the assignment (e.g., `var list = new List<string>();`). Use explicit types for method returns or literals (e.g., `int count = 5;`).
* **Expression-Bodied Members:** Use `=>` for simple one-line methods and properties.
* **Top-Level Statements:** Use Top-Level Statements for entry points (Program.cs).

## 3. Performance & Safety
* **Span Optimization:** Prefer `ReadOnlySpan<char>` for string parsing and slicing.
* **Required Members:** Use the `required` keyword for properties that must be initialized via object initializers instead of bloating constructors.
* **Raw String Literals:** Use `"""` for multi-line strings or JSON to avoid escaping quotes.

## 4. Naming Conventions
* **PascalCase:** Classes, Methods, Properties, Public Fields.
* **camelCase:** Local variables, method arguments.
* **No Underscores:** Avoid `_` prefixes for private fields (since we use the `field` keyword or `this.` prefix if absolutely necessary).


## For Claude

Each time we complete the changes, we need to use 'dotnet build' to test and validate the changes worked.
When making changes, please ensure that the code is well-structured, follows best practices, and includes appropriate error handling. 
If new information is needed to complete the task, please ask for clarification before proceeding.
After completing the changes, please consider making changes to CLAUDE.md and or README.md to reflect the changes made and to provide clear documentation for future reference.
