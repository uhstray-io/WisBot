Console.WriteLine("Starting bot...");

var terminal = new Terminal();
var bot = new Bot(terminal);

// Connect them bidirectionally
terminal.Bot = bot;

// Propagate unhandled exceptions from fire-and-forget tasks so failures are visible.
// Without this, an exception in StartBot (bad token, network failure, etc.) is silently
// swallowed — the process stays alive but the bot never connects.
TaskScheduler.UnobservedTaskException += (_, e) => {
    Console.Error.WriteLine($"[FATAL] Unobserved task exception: {e.Exception}");
    e.SetObserved(); // prevent process crash from unrelated background errors
};

_ = terminal.NewTerminal();
_ = bot.StartBot().ContinueWith(t => {
    if (t.IsFaulted) {
        Console.Error.WriteLine($"[FATAL] Bot failed to start: {t.Exception?.GetBaseException().Message}");
        Environment.Exit(1);
    }
}, TaskContinuationOptions.OnlyOnFaulted);

await Task.Delay(-1);
