using WisBot;

Console.WriteLine("Starting bot...");

  var terminal = new Terminal();                                                                                                                   
  var bot = new Bot(terminal);
  
  terminal.Bot = bot;

  var botTask = bot.StartBot();
  var terminalTask = terminal.NewTerminal();

  // Bot failure = fatal; terminal crash = warn and continue
  var completed = await Task.WhenAny(botTask, terminalTask);

  if (completed == botTask && botTask.IsFaulted)
  {
      Console.Error.WriteLine($"[FATAL] Bot failed: {botTask.Exception?.GetBaseException().Message}");
      Environment.Exit(1);
  }

  if (terminalTask.IsFaulted)
      Console.Error.WriteLine($"[WARN] Terminal crashed: {terminalTask.Exception?.GetBaseException().Message}");

  await Task.Delay(-1);
