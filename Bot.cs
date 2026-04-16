using Discord;
using Discord.WebSocket;

public class Bot(Terminal terminal) {
    private const ulong uhstrayGuildId = 889910011113906186;

    private string token = null!;

    private DiscordSocketClient client = null!;
    private VoiceRecorder voiceRecorder = new VoiceRecorder(terminal);
    private WelcomeHandler welcomeHandler = new WelcomeHandler(terminal);
    private UserVoiceActivityTracker voiceActivityTracker = new UserVoiceActivityTracker(terminal);
    private VoiceStatsHandler voiceStatsHandler = new VoiceStatsHandler(terminal);
    private WisLlmHandler wisLlmHandler = new WisLlmHandler(terminal);
    private ReminderService? reminderService;
    private VoiceNotificationHandler? voiceNotifyHandler;
    private StatusHandler? statusHandler;

    public async Task StartBot() {
        var config = new DiscordSocketConfig {
            GatewayIntents =
                GatewayIntents.AllUnprivileged
                | GatewayIntents.MessageContent
                | GatewayIntents.GuildMessages
                | GatewayIntents.GuildMembers,
        };

        client = new DiscordSocketClient(config);
        reminderService = new ReminderService(terminal, client);
        voiceNotifyHandler = new VoiceNotificationHandler(terminal, client);
        statusHandler = new StatusHandler(terminal, client);
        client.Log += OnLog;
        client.MessageUpdated += OnMessageUpdated;
        client.MessageReceived += OnMessageReceived;
        client.Ready += OnReady;
        client.UserJoined += welcomeHandler.OnUserJoined;
        client.UserVoiceStateUpdated += voiceNotifyHandler.OnVoiceStateUpdated;
        client.UserVoiceStateUpdated += voiceActivityTracker.OnVoiceStateUpdated;
        client.UserIsTyping += OnUserIsTyping;
        client.SlashCommandExecuted += OnSlashCommandExecuted;

        await InitBot();
        await client.LoginAsync(TokenType.Bot, token);
        await client.StartAsync();
    }

    private async Task InitBot() {
        Config.Load();
        await Log("Reading discord key from file...");
        var content = await File.ReadAllTextAsync("discord.key");
        token = content.Trim();
    }



    private async Task OnUserIsTyping(Cacheable<IUser, ulong> cachedUser, Cacheable<IMessageChannel, ulong> cachedChannel) {
        var user = await cachedUser.GetOrDownloadAsync();
        var channel = await cachedChannel.GetOrDownloadAsync();
        await Log($"{user} is typing in {channel}...");
    }


    private async Task OnMessageReceived(SocketMessage message) {
        if (message.Author.IsBot) {
            return;
        }

        await Log($"[{message.Channel.Name}] {message.Author}: {message.Content}");

        if (message.Content.StartsWith("!")) {
            if (message.Content.StartsWith("!eatdeeznuts")) {
                if (Random.Shared.NextDouble() < 0.1) {
                    _ = message.Channel.SendMessageAsync("Sure, I could go for some nuts.");
                    return;
                }
                _ = message.Channel.SendMessageAsync("No thanks, I'm good.");
                return;
            }
            _ = message.Channel.SendMessageAsync("Command detected, but commands are not implemented yet.");
            return;
        }
    }

    private async Task OnReady() {
        await Log("Bot is Ready!!!");

        await Database.Initialize();
        await reminderService!.Start();
        await AddCommandsIfNotExist();
    }

    public async Task AddCommandsIfNotExist() {
        await Log("Registering slash commands");

        var guild = client.GetGuild(uhstrayGuildId);
        var existingGuildCommands = await guild.GetApplicationCommandsAsync();

        // Uhstray guild commands

        if (!existingGuildCommands.Any(cmd => cmd.Name == "recording")) {
            var command = new SlashCommandBuilder()
                .WithName("recording")
                .WithDescription("Control voice channel recording")
                .AddOption(new SlashCommandOptionBuilder()
                    .WithName("action")
                    .WithDescription("Start or stop recording")
                    .WithRequired(true)
                    .WithType(ApplicationCommandOptionType.String)
                    .AddChoice("Start", "start")
                    .AddChoice("Stop", "stop")
                )
                .AddOption(new SlashCommandOptionBuilder()
                    .WithName("sendfile")
                    .WithDescription("Send files directly in Discord chat (default: false)")
                    .WithRequired(false)
                    .WithType(ApplicationCommandOptionType.Boolean)
                )
                .AddOption(new SlashCommandOptionBuilder()
                    .WithName("mergeaudio")
                    .WithDescription("Merge all audio sources into one file (default: false)")
                    .WithRequired(false)
                    .WithType(ApplicationCommandOptionType.Boolean)
                )
                .Build();

            await guild.CreateApplicationCommandAsync(command);
            await Log($"Registered guild slash command: /{command.Name} in '{guild.Name}'");
        }




        if (!existingGuildCommands.Any(cmd => cmd.Name == "remind")) {
            var command = new SlashCommandBuilder()
                .WithName("remind")
                .WithDescription("Set a reminder — bot will DM you when time is up")
                .AddOption(new SlashCommandOptionBuilder()
                    .WithName("when")
                    .WithDescription("How long from now, e.g. 30m, 2h, 1d, 1h30m")
                    .WithRequired(true)
                    .WithType(ApplicationCommandOptionType.String)
                )
                .AddOption(new SlashCommandOptionBuilder()
                    .WithName("message")
                    .WithDescription("What to remind you about")
                    .WithRequired(true)
                    .WithType(ApplicationCommandOptionType.String)
                )
                .Build();

            await guild.CreateApplicationCommandAsync(command);
            await Log($"Registered guild slash command: /{command.Name} in '{guild.Name}'");
        }

        if (!existingGuildCommands.Any(cmd => cmd.Name == "status")) {
            var command = new SlashCommandBuilder()
                .WithName("status")
                .WithDescription("Show bot health: uptime, latency, memory, CPU, and more")
                .Build();

            await guild.CreateApplicationCommandAsync(command);
            await Log($"Registered guild slash command: /{command.Name} in '{guild.Name}'");
        }

        if (!existingGuildCommands.Any(cmd => cmd.Name == "voicestats")) {
            var command = new SlashCommandBuilder()
                .WithName("voicestats")
                .WithDescription("Show voice channel stats for a user")
                .AddOption(new SlashCommandOptionBuilder()
                    .WithName("user")
                    .WithDescription("The user to look up")
                    .WithRequired(true)
                    .WithType(ApplicationCommandOptionType.User)
                )
                .Build();

            await guild.CreateApplicationCommandAsync(command);
            await Log($"Registered guild slash command: /{command.Name} in '{guild.Name}'");
        }

        if (!existingGuildCommands.Any(cmd => cmd.Name == "notify")) {
            var command = new SlashCommandBuilder()
                .WithName("notify")
                .WithDescription("DM me once when a user joins a voice channel")
                .AddOption(new SlashCommandOptionBuilder()
                    .WithName("user")
                    .WithDescription("The user to watch for")
                    .WithRequired(true)
                    .WithType(ApplicationCommandOptionType.User)
                )
                .Build();

            await guild.CreateApplicationCommandAsync(command);
            await Log($"Registered guild slash command: /{command.Name} in '{guild.Name}'");
        }

        // Global commands
        var existingGlobalCommands = await client.GetGlobalApplicationCommandsAsync();

        // Replace old single-option wisllm with subcommand version if needed
        var existingWisllm = existingGlobalCommands.FirstOrDefault(cmd => cmd.Name == "wisllm");
        bool wisllmNeedsUpdate = existingWisllm == null ||
            !existingWisllm.Options.Any(o => o.Type == ApplicationCommandOptionType.SubCommand);

        if (wisllmNeedsUpdate) {
            if (existingWisllm != null) {
                await existingWisllm.DeleteAsync();
                await Log("Deleted outdated /wisllm global command — replacing with subcommand version");
            }

            var command = new SlashCommandBuilder()
                .WithName("wisllm")
                .WithDescription("WisLLM — AI assistant provided by Uhstray.io")
                .AddOption(new SlashCommandOptionBuilder()
                    .WithName("ask")
                    .WithDescription("Ask WisLLM a question")
                    .WithType(ApplicationCommandOptionType.SubCommand)
                    .AddOption("prompt", ApplicationCommandOptionType.String, "Your question", isRequired: true)
                    .AddOption("model",  ApplicationCommandOptionType.String, "Model to use (default from config)", isRequired: false)
                )
                .AddOption(new SlashCommandOptionBuilder()
                    .WithName("clear")
                    .WithDescription("Start a fresh conversation session")
                    .WithType(ApplicationCommandOptionType.SubCommand)
                )
                .AddOption(new SlashCommandOptionBuilder()
                    .WithName("compact")
                    .WithDescription("Summarise the current session into a single context and continue from it")
                    .WithType(ApplicationCommandOptionType.SubCommand)
                )
                .Build();

            await client.CreateGlobalApplicationCommandAsync(command);
            await Log($"Registered global slash command: /{command.Name} (ask / clear / compact)");
        }
    }


    private async Task OnSlashCommandExecuted(SocketSlashCommand command) {
        // Log User and command info
        await Log($"Slash command executed: {command.CommandName} by {command.User.Username}");
        await Log($"Command options: {string.Join(", ", command.Data.Options.Select(opt => $"{opt.Name}={opt.Value}"))}");


        if (command.CommandName == "remind") {
            await reminderService!.HandleRemindCommand(command);
            return;
        }

        if (command.CommandName == "status") {
            await statusHandler!.HandleCommand(command);
            return;
        }

        if (command.CommandName == "voicestats") {
            await voiceStatsHandler.HandleCommand(command);
            return;
        }

        if (command.CommandName == "notify") {
            await voiceNotifyHandler!.HandleNotifyCommand(command);
            return;
        }

        if (command.CommandName == "wisllm") {
            var sub = command.Data.Options.First().Name;
            switch (sub) {
                case "ask":     await wisLlmHandler.HandleAskCommand(command);     break;
                case "clear":   await wisLlmHandler.HandleClearCommand(command);   break;
                case "compact": await wisLlmHandler.HandleCompactCommand(command); break;
            }
            return;
        }

        if (command.CommandName == "recording") {
            await voiceRecorder.HandleRecordingCommand(command);
            return;
        }
    }

    public async Task RemoveAllCommands() {
        await Log("Removing all existing commands...");

        try {
            // Remove global commands
            await Log("Removing global commands");
            var globalCommands = await client.GetGlobalApplicationCommandsAsync();
            foreach (var command in globalCommands) {
                await command.DeleteAsync();
                await Log($"  Deleted global command: {command.Name}");
                await Task.Delay(100);
            }

            // Remove guild-specific commands for the configured guild
            await Log($"Removing guild commands for guild {uhstrayGuildId}");
            var guild = client.GetGuild(uhstrayGuildId);
            if (guild != null) {
                var guildCommands = await guild.GetApplicationCommandsAsync();
                foreach (var command in guildCommands) {
                    await command.DeleteAsync();
                    await Log($"  Deleted guild command: {command.Name}");
                    await Task.Delay(100);
                }
            }

            // Optionally: Remove commands from all guilds the bot is in
            await Log("Removing commands from all guilds");
            foreach (var g in client.Guilds) {
                var commands = await g.GetApplicationCommandsAsync();
                foreach (var command in commands) {
                    await command.DeleteAsync();
                    await Log($"  Deleted command '{command.Name}' from guild {g.Name}");
                    await Task.Delay(100);
                }
            }

            await Log("Successfully removed all commands!");
        } catch (Exception ex) {
            await Log($"Error removing commands: {ex.Message}");
        }
    }



    /// Terminal command: joins a hardcoded voice channel, records for 15 seconds, then stops and saves.
    public async Task TestRecord() {
        const ulong testGuildId = 889910011113906186;
        const ulong testChannelId = 889910012019867672;

        var guild = client.GetGuild(testGuildId);
        if (guild == null) {
            await Log("❌ TestRecord: Could not find guild");
            return;
        }

        var channel = guild.GetVoiceChannel(testChannelId);
        if (channel == null) {
            await Log("❌ TestRecord: Could not find voice channel");
            return;
        }

        await Log($"🎙️ TestRecord: Joining {channel.Name} in {guild.Name}...");
        var joinResult = await voiceRecorder.JoinAndRecordChannel(channel);
        await Log($"🎙️ TestRecord: {joinResult}");

        await Log("🎙️ TestRecord: Recording for 15 seconds...");
        await Task.Delay(TimeSpan.FromSeconds(15));

        await Log("🎙️ TestRecord: Stopping recording...");
        var files = await voiceRecorder.StopRecordingAndSave();

        if (files.Count > 0) {
            await Log($"🎙️ TestRecord: Complete! Saved {files.Count} file(s):");
            foreach (var file in files) {
                await Log($"  📁 {file}");
            }
        } else {
            await Log("🎙️ TestRecord: Complete, but no audio was captured.");
        }
    }

    private async Task OnMessageUpdated(Cacheable<IMessage, ulong> before, SocketMessage after, ISocketMessageChannel channel) {
        var message = await before.GetOrDownloadAsync();
        await Log($"Message From {message.Author.Username} updated from '{message}' to '{after}' in channel {channel.Name}");
    }


    private async Task OnLog(LogMessage msg) {
        await Log(msg.ToString());
    }

    private async Task Log(string msg) {
        await terminal.AddLine($"[Discord] {msg}");
    }
}