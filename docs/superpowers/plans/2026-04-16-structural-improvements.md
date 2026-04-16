# Structural Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve WisBot's codebase structure with folders, namespaces, consolidated config, log levels, and a command registry.

**Architecture:** Five independent, incremental changes — each leaves the project in a buildable state. No test framework exists; `dotnet build` (0 warnings, 0 errors) is the verification step after every task.

**Tech Stack:** C# 14, .NET 10, Discord.Net 3.19, Microsoft.Data.Sqlite, SDK-style csproj (auto-includes all `**/*.cs`)

---

## File Map

| Task | Files Modified | Files Created |
|------|---------------|---------------|
| 1 — Folder structure | All feature .cs files (move) | `Features/` directory |
| 2 — Namespaces | All .cs files | — |
| 3 — Config consolidation | `Config.cs`, `Bot.cs` | — |
| 4 — Log levels | `Terminal.cs`, all handler files | — |
| 5 — Command registry | `Bot.cs` | — |

---

## Task 1: Move feature files into Features/

**Why:** 13 files flat at root is readable now, but the project is growing. Separating infrastructure from feature handlers makes navigation faster and scope clearer.

**Files:**
- Create: `Features/` directory
- Move: `VoiceRecorder.cs` → `Features/VoiceRecorder.cs`
- Move: `UserVoiceActivityTracker.cs` → `Features/UserVoiceActivityTracker.cs`
- Move: `VoiceStatsHandler.cs` → `Features/VoiceStatsHandler.cs`
- Move: `VoiceNotificationHandler.cs` → `Features/VoiceNotificationHandler.cs`
- Move: `WelcomeHandler.cs` → `Features/WelcomeHandler.cs`
- Move: `ReminderService.cs` → `Features/ReminderService.cs`
- Move: `StatusHandler.cs` → `Features/StatusHandler.cs`
- Move: `WisLlmHandler.cs` → `Features/WisLlmHandler.cs`
- Stay: `Program.cs`, `Bot.cs`, `Config.cs`, `Database.cs`, `Terminal.cs`

- [ ] **Step 1: Create Features/ directory and move files**

```bash
mkdir Features
mv VoiceRecorder.cs Features/
mv UserVoiceActivityTracker.cs Features/
mv VoiceStatsHandler.cs Features/
mv VoiceNotificationHandler.cs Features/
mv WelcomeHandler.cs Features/
mv ReminderService.cs Features/
mv StatusHandler.cs Features/
mv WisLlmHandler.cs Features/
```

> Note: SDK-style `.csproj` automatically includes all `**/*.cs` — no project file changes needed.

- [ ] **Step 2: Verify build**

```bash
dotnet build
```
Expected: `Build succeeded. 0 Warning(s) 0 Error(s)`

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "refactor: move feature handlers into Features/"
```

---

## Task 2: Add namespaces

**Why:** Everything is currently in the global namespace. As the project grows, name collisions become possible and discoverability suffers. Use a single flat `WisBot` namespace — no sub-namespaces — to avoid cross-namespace `using` chains.

**Files:**
- Modify: every `.cs` file — add `namespace WisBot;` as the first non-comment line (file-scoped namespace, no curly braces)

- [ ] **Step 1: Add namespace to core files**

Add `namespace WisBot;` as the first line (after any `using` statements) to each of these files:

`Bot.cs`:
```csharp
using Discord;
using Discord.WebSocket;

namespace WisBot;

public class Bot(Terminal terminal) {
```

`Config.cs`:
```csharp
namespace WisBot;

public static class Config {
```

`Database.cs`:
```csharp
using Microsoft.Data.Sqlite;

namespace WisBot;

public static class Database {
```

`Terminal.cs`:
```csharp
using System.Collections.Concurrent;

namespace WisBot;

public class Terminal {
```

- [ ] **Step 2: Add namespace to all Features/ files**

Add `namespace WisBot;` to each file in `Features/`. Example for `Features/ReminderService.cs`:
```csharp
using Discord.WebSocket;
using Microsoft.Data.Sqlite;

namespace WisBot;

public record Reminder {
```

Repeat for: `VoiceRecorder.cs`, `UserVoiceActivityTracker.cs`, `VoiceStatsHandler.cs`, `VoiceNotificationHandler.cs`, `WelcomeHandler.cs`, `StatusHandler.cs`, `WisLlmHandler.cs`.

> `Program.cs` uses top-level statements — no namespace declaration needed.

- [ ] **Step 3: Verify build**

```bash
dotnet build
```
Expected: `Build succeeded. 0 Warning(s) 0 Error(s)`

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor: add WisBot namespace to all files"
```

---

## Task 3: Consolidate config — Discord token into Config.cs

**Why:** `Bot.cs` reads `discord.key` directly. `Config.cs` owns all other external configuration. These should be consistent — `Config` is the single place to look for any runtime setting.

**Files:**
- Modify: `Config.cs` — add `DiscordToken` property and load from `discord.key`
- Modify: `Bot.cs` — remove `InitBot`'s token reading, use `Config.DiscordToken`

- [ ] **Step 1: Add DiscordToken to Config.cs**

Add a `DiscordToken` property and load logic. The full updated `Config.cs`:

```csharp
namespace WisBot;

/// Loads configuration from a .env file and discord.key at startup.
/// Falls back to safe defaults if the file or a key is missing.
public static class Config {
    public static string DiscordToken       { get; private set; } = string.Empty;
    public static string OllamaEndpoint     { get; private set; } = "http://localhost:11434";
    public static string OllamaDefaultModel { get; private set; } = "llama3";
    public static int    WisLlmContextLimit { get; private set; } = 10;
    public static int    WisLlmWarnAtPercent{ get; private set; } = 75;
    public static int    WisLlmContextSize  { get; private set; } = 0;

    public static void Load(string envPath = ".env", string tokenPath = "discord.key") {
        // discord.key is the local dev path; .env is the Docker/CI path
        if (File.Exists(tokenPath))
            DiscordToken = File.ReadAllText(tokenPath).Trim();

        if (!File.Exists(envPath)) return;

        foreach (var raw in File.ReadAllLines(envPath)) {
            var line = raw.Trim();
            if (line.StartsWith('#') || !line.Contains('=')) continue;

            int idx   = line.IndexOf('=');
            var key   = line[..idx].Trim();
            var value = line[(idx + 1)..].Trim();

            switch (key) {
                // Deployment workflow (deployment_prod.yml) appends DISCORD_TOKEN_WISBOT to .env
                case "DISCORD_TOKEN_WISBOT":
                    if (!string.IsNullOrWhiteSpace(value)) DiscordToken = value;
                    break;
                case "OLLAMA_ENDPOINT":        OllamaEndpoint     = value; break;
                case "OLLAMA_DEFAULT_MODEL":   OllamaDefaultModel = value; break;
                case "WISLLM_CONTEXT_LIMIT":
                    if (int.TryParse(value, out int limit) && limit > 0)
                        WisLlmContextLimit = limit;
                    break;
                case "WISLLM_WARN_AT_PERCENT":
                    if (int.TryParse(value, out int pct) && pct is > 0 and <= 100)
                        WisLlmWarnAtPercent = pct;
                    break;
                case "WISLLM_CONTEXT_SIZE":
                    if (int.TryParse(value, out int size) && size > 0)
                        WisLlmContextSize = size;
                    break;
            }
        }
    }
}
```

> `discord.key` is used for local dev. Docker CI appends `DISCORD_TOKEN_WISBOT` to `.env` — that overrides the key file.

- [ ] **Step 2: Update Bot.cs to use Config.DiscordToken**

Replace `InitBot` in `Bot.cs`:

```csharp
private async Task InitBot() {
    Config.Load();
    if (string.IsNullOrWhiteSpace(Config.DiscordToken))
        throw new InvalidOperationException(
            "Discord token not found. Add it to discord.key or set DISCORD_TOKEN in .env");
    token = Config.DiscordToken;
    await terminal.AddLine("[Bot] Config loaded");
}
```

- [ ] **Step 3: Verify build**

```bash
dotnet build
```
Expected: `Build succeeded. 0 Warning(s) 0 Error(s)`

- [ ] **Step 4: Verify runtime behavior**

Run the bot. Confirm it connects normally — token loading path unchanged for `discord.key` users.

- [ ] **Step 5: Commit**

```bash
git add Features/ Config.cs Bot.cs
git commit -m "refactor(config): consolidate Discord token into Config"
```

---

## Task 4: Add log levels to Terminal

**Why:** Every `terminal.AddLine` call currently looks identical — debug noise and real errors are indistinguishable. Adding `Info`/`Warn`/`Error` levels makes production logs scannable.

**Files:**
- Modify: `Terminal.cs` — add `LogLevel` enum, update `AddLine` signature
- Modify: All handler files — update error `Log()` calls to pass `LogLevel.Error` or `LogLevel.Warn`

- [ ] **Step 1: Add LogLevel enum and update Terminal.cs**

Replace the current `AddLine` method. Full updated `Terminal.cs`:

```csharp
using System.Collections.Concurrent;

namespace WisBot;

public enum LogLevel { Info, Warn, Error }

public class Terminal {
    ConcurrentQueue<string> lines = new ConcurrentQueue<string>();

    public Bot? Bot { get; set; }

    public async Task NewTerminal() {
        while (true) {
            var element = await Task.Run(() => Console.ReadLine());
            if (element == null) continue;
            Console.Write("\x1b[1A\x1b[2K");
            await ProcessInput(element);
        }
    }

    public async Task AddLine(string line, LogLevel level = LogLevel.Info) {
        string formatted = level switch {
            LogLevel.Warn  => $"[WARN]  {line}",
            LogLevel.Error => $"[ERROR] {line}",
            _              =>            line,
        };
        lines.Enqueue(formatted);
        await Console.Out.WriteLineAsync(formatted);
    }

    public async Task AddLines(List<string> newLines) {
        foreach (var line in newLines)
            await AddLine(line);
    }

    private async Task ProcessInput(string element) {
        var task = element switch {
            "help" or "?" or "/help" => AddLines([
                "Available commands:",
                "/help or ? - Show this help message",
                "/clear - Clear the terminal",
                "/removeallcommands - Remove all Discord bot commands",
                "/testrecord - Join voice channel, record 15s, save and leave",
            ]),
            "/clear"             => ClearTerminal(),
            "/removeallcommands" => Bot?.RemoveAllCommands() ?? Log("Bot not initialized yet"),
            "/testrecord"        => Bot?.TestRecord()        ?? Log("Bot not initialized yet"),
            "/gc"                => RunGC(),
            _                    => Log(element),
        };
        await task;
    }

    private Task ClearTerminal() {
        Console.Write("\x1b[2J\x1b[H\x1b[3J");
        lines.Clear();
        return Task.CompletedTask;
    }

    private async Task Log(string message) => await AddLine($"[Terminal] {message}");

    private async Task RunGC() {
        GC.Collect();
        GC.WaitForPendingFinalizers();
        await AddLine("[Terminal] Garbage collection completed");
    }
}
```

- [ ] **Step 2: Update error log calls in handlers**

Each handler has a `private async Task Log(string msg)` helper. Change the catch-block calls in each file to pass `LogLevel.Error`. The pattern to find and update:

In `Features/ReminderService.cs` — update Deliver():
```csharp
await Log($"DM failed for reminder {reminder.Id}: {ex.Message}", LogLevel.Error);
// ...
await Log($"Channel fallback failed for reminder {reminder.Id}: {ex.Message}", LogLevel.Error);
// ...
await Log(sent
    ? $"Delivered reminder {reminder.Id} to user {reminder.UserId}"
    : $"Could not deliver reminder {reminder.Id} — dropped", sent ? LogLevel.Info : LogLevel.Warn);
```

Update the `Log` helper signature in every handler to pass the level through:
```csharp
private async Task Log(string msg, LogLevel level = LogLevel.Info)
    => await terminal.AddLine($"[Reminders] {msg}", level);
```

Apply the same pattern to all handlers: `WisLlmHandler`, `VoiceNotificationHandler`, `VoiceStatsHandler`, `UserVoiceActivityTracker`, `WelcomeHandler`, `StatusHandler`, `VoiceRecorder`, `Bot`.

For each handler, update:
1. The `Log` helper signature to accept `LogLevel level = LogLevel.Info`
2. The `Log` helper body to pass `level` to `terminal.AddLine`
3. Any catch block that calls `Log(...)` to pass `LogLevel.Error`
4. Any "not found / could not deliver" calls to pass `LogLevel.Warn`

- [ ] **Step 3: Verify build**

```bash
dotnet build
```
Expected: `Build succeeded. 0 Warning(s) 0 Error(s)`

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(terminal): add Info/Warn/Error log levels"
```

---

## Task 5: Command registry in Bot.cs

**Why:** `OnSlashCommandExecuted` is an if-chain that grows by 4+ lines per new command. A dictionary maps command names to handlers — adding a new command is one line, not four.

**Files:**
- Modify: `Bot.cs` — replace if-chain with `Dictionary<string, Func<SocketSlashCommand, Task>>`

- [ ] **Step 1: Add commandHandlers dictionary to Bot.cs**

Add this field after the service fields:

```csharp
private Dictionary<string, Func<SocketSlashCommand, Task>> commandHandlers = [];
```

- [ ] **Step 2: Initialize registry in OnReady**

In `OnReady`, after `AddCommandsIfNotExist()`, initialize the registry:

```csharp
private async Task OnReady() {
    await terminal.AddLine("[Bot] Bot is Ready!!!");
    await Database.Initialize();
    await reminderService!.Start();
    await AddCommandsIfNotExist();

    commandHandlers = new Dictionary<string, Func<SocketSlashCommand, Task>> {
        ["remind"]     = cmd => reminderService!.HandleRemindCommand(cmd),
        ["status"]     = cmd => statusHandler!.HandleCommand(cmd),
        ["voicestats"] = cmd => voiceStatsHandler.HandleCommand(cmd),
        ["notify"]     = cmd => voiceNotifyHandler!.HandleNotifyCommand(cmd),
        ["recording"]  = cmd => voiceRecorder.HandleRecordingCommand(cmd),
        ["wisllm"]     = HandleWisLlmCommand,
    };
}
```

- [ ] **Step 3: Extract HandleWisLlmCommand**

Extract the wisllm sub-command switch into its own method (keeps the registry entries flat):

```csharp
private async Task HandleWisLlmCommand(SocketSlashCommand command) {
    var sub = command.Data.Options.First().Name;
    switch (sub) {
        case "ask":     await wisLlmHandler.HandleAskCommand(command);     break;
        case "clear":   await wisLlmHandler.HandleClearCommand(command);   break;
        case "compact": await wisLlmHandler.HandleCompactCommand(command); break;
    }
}
```

- [ ] **Step 4: Replace OnSlashCommandExecuted**

Replace the existing method body entirely:

```csharp
private async Task OnSlashCommandExecuted(SocketSlashCommand command) {
    await terminal.AddLine($"[Bot] /{command.CommandName} by {command.User.Username}");

    if (commandHandlers.TryGetValue(command.CommandName, out var handler))
        await handler(command);
    else
        await terminal.AddLine($"[Bot] Unknown command: {command.CommandName}", LogLevel.Warn);
}
```

- [ ] **Step 5: Remove the old log line in OnSlashCommandExecuted**

The old method also logged options separately:
```csharp
await Log($"Command options: {string.Join(", ", command.Data.Options.Select(opt => $"{opt.Name}={opt.Value}"))}");
```
This is now removed — it logged sub-command names as option values, which was noisy and misleading.

- [ ] **Step 6: Verify build**

```bash
dotnet build
```
Expected: `Build succeeded. 0 Warning(s) 0 Error(s)`

- [ ] **Step 7: Commit**

```bash
git add Features/ Bot.cs
git commit -m "refactor(bot): replace command if-chain with registry dictionary"
```
