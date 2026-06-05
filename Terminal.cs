using System.Collections.Concurrent;

namespace WisBot;

public enum LogLevel { Info, Warn, Error }

// Terminal class to manage terminal output and user input
// The terminal maintains and orchistrates actions between different parts of the program
public class Terminal {
    ConcurrentQueue<string> lines = new();

    public Bot? Bot { get; set; }

    // Start a new terminal session
    public async Task NewTerminal() {
        while (true) {
            var element = await Task.Run(() => Console.ReadLine());
            if (element == null)
                continue;

            // Move cursor up one line and clear it to erase the raw input
            Console.Write("\x1b[1A\x1b[2K");
            await ProcessInput(element);
        }
    }

    public async Task AddLine(string line, LogLevel level = LogLevel.Info) {
        string formatted = level switch {
            LogLevel.Warn => $"[WARN]  {line}",
            LogLevel.Error => $"[ERROR] {line}",
            _ => line,
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
                "/gc - Force a .NET garbage collection",
            ]),
            "/clear" => ClearTerminal(),
            "/removeallcommands" => Bot?.RemoveAllCommands() ?? Log("Bot not initialized yet"),
            "/testrecord" => Bot?.TestRecord() ?? Log("Bot not initialized yet"),
            "/gc" => RunGC(),
            _ => Log(element),
        };
        await task;
    }

    private Task ClearTerminal() {
        // \x1b[2J clears the screen, \x1b[H moves cursor to top-left, \x1b[3J clears the scrollback buffer
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
