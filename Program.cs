Console.WriteLine("Starting bot...");

var terminal = new Terminal();
var bot = new Bot(terminal);

// Connect them bidirectionally
terminal.Bot = bot;

_ = terminal.NewTerminal();
_ = bot.StartBot();

await Task.Delay(-1);
