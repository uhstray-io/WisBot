using System.Runtime.InteropServices;
using WisBot;

Console.WriteLine("Starting bot...");

var terminal = new Terminal();
var bot = new Bot(terminal);

terminal.Bot = bot;

// Signal-driven shutdown: SIGTERM (container stop) and SIGINT (Ctrl+C) trigger a graceful stop.
var shutdown = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
using var sigterm = PosixSignalRegistration.Create(PosixSignal.SIGTERM, ctx => { ctx.Cancel = true; shutdown.TrySetResult(); });
using var sigint = PosixSignalRegistration.Create(PosixSignal.SIGINT, ctx => { ctx.Cancel = true; shutdown.TrySetResult(); });

var botTask = bot.StartBot();
var terminalTask = terminal.NewTerminal();

// Bot startup failure = fatal; terminal crash = warn and continue.
// Note: botTask completes successfully once the gateway connect has kicked off —
// it does not represent the bot's lifetime. The shutdown TCS does.
await Task.WhenAny(botTask, terminalTask);

if (botTask.IsFaulted) {
    Console.Error.WriteLine($"[FATAL] Bot failed: {botTask.Exception?.GetBaseException().Message}");
    Environment.Exit(1);
}

if (terminalTask.IsFaulted)
    Console.Error.WriteLine($"[WARN] Terminal crashed: {terminalTask.Exception?.GetBaseException().Message}");

await shutdown.Task;

Console.WriteLine("Shutting down...");
var stop = bot.StopBot();
if (await Task.WhenAny(stop, Task.Delay(TimeSpan.FromSeconds(10))) != stop)
    Console.Error.WriteLine("[WARN] Graceful shutdown timed out after 10s");
