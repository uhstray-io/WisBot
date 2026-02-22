using System.Collections.Concurrent;


// Terminal class to manage terminal output and user input
// The terminal maintains and orchistrates actions between different parts of the program
public class Terminal {
    ConcurrentQueue<string> _lines = new ConcurrentQueue<string>();
    private readonly int maxLines = 1_000;

    private int terminalWidth = 80;
    private int terminalHeight = 25;

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

    private void GetTerminalSize() {
        try {
            terminalWidth = Console.WindowWidth;
            terminalHeight = Console.WindowHeight;
        } catch (Exception) {
            terminalWidth = 80;
            terminalHeight = 25;
        }
    }

    private async Task AllWriteLines() {
        // \x1b[2J clears the screen, \x1b[H moves cursor to top-left, \x1b[3J clears the scrollback buffer
        Console.Write("\x1b[2J\x1b[H\x1b[3J");
        GetTerminalSize();

        _lines = new ConcurrentQueue<string>(_lines.TakeLast(maxLines));

        foreach (var line in _lines) {
            await Console.Out.WriteLineAsync($"{line}");
        }
    }

    private async Task WriteLine(string line) {
        await Console.Out.WriteLineAsync(line);
    }

    public async Task AddLine(string line) {
        _lines.Enqueue(line);
        await WriteLine(line);
    }

    public async Task AddLines(List<string> lines) {
        foreach (var line in lines) {
            _lines.Enqueue(line);
            await WriteLine(line);
        }
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
            "/clear" => ClearTerminal(),
            "/removeallcommands" => Bot?.RemoveAllCommands() ?? Log("Bot not initialized yet"),
            "/testrecord" => Bot?.TestRecord() ?? Log("Bot not initialized yet"),
            "/gc" => RunGC(),
            _ => Log($"{element}"),
        };
        await task;
    }

    private async Task ClearTerminal() {
        // \x1b[2J clears the screen, \x1b[H moves cursor to top-left, \x1b[3J clears the scrollback buffer
        Console.Write("\x1b[2J\x1b[H\x1b[3J");
        _lines.Clear();
    }

    private async Task Log(string message) {
        await AddLine($"[Terminal] {message}");
    }

    private async Task RunGC() {
        GC.Collect();
        GC.WaitForPendingFinalizers();
        await AddLine("[Terminal] Garbage collection completed");
    }
}
