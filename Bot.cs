using Discord;
using Discord.WebSocket;

namespace WisBot;

public class Bot(Terminal terminal) {
    private string token = null!;

    private DiscordSocketClient client = null!;
    private VoiceRecorder voiceRecorder = new(terminal);
    private WelcomeService welcomeService = new(terminal);
    private UserVoiceActivityTracker voiceActivityTracker = new(terminal);
    private VoiceStatsService voiceStatsService = new(terminal);
    private WisLlmService wisLlmService = new(terminal);
    private ReminderService? reminderService;
    private VoiceNotificationService? voiceNotifyService;
    private StatusService? statusService;
    private Dictionary<string, Func<SocketSlashCommand, Task>> commands = [];

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
        voiceNotifyService = new VoiceNotificationService(terminal, client);
        statusService = new StatusService(terminal, client);
        client.Log += OnLog;
        client.MessageUpdated += OnMessageUpdated;
        client.MessageReceived += OnMessageReceived;
        client.Ready += OnReady;
        client.UserJoined += welcomeService.OnUserJoined;
        client.UserVoiceStateUpdated += voiceNotifyService.OnVoiceStateUpdated;
        client.UserVoiceStateUpdated += voiceActivityTracker.OnVoiceStateUpdated;
        client.UserIsTyping += OnUserIsTyping;
        client.SlashCommandExecuted += OnSlashCommandExecuted;

        await InitBot();
        await client.LoginAsync(TokenType.Bot, token);
        await client.StartAsync();
    }

    private async Task InitBot() {
        Config.Load();
        if (string.IsNullOrWhiteSpace(Config.DiscordToken))
            throw new InvalidOperationException(
                "Discord token not found. Add it to discord.key or set DISCORD_TOKEN_WISBOT in .env");
        if (Config.GuildId == 0)
            throw new InvalidOperationException(
                "WISBOT_GUILD_ID not set. Set the Discord guild the bot serves via environment or .env");
        token = Config.DiscordToken;
        await terminal.AddLine("[Bot] Config loaded");
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

        commands = new Dictionary<string, Func<SocketSlashCommand, Task>> {
            ["remind"] = cmd => reminderService!.HandleRemindCommand(cmd),
            ["status"] = cmd => statusService!.HandleCommand(cmd),
            ["voicestats"] = cmd => voiceStatsService.HandleCommand(cmd),
            ["notify"] = cmd => voiceNotifyService!.HandleNotifyCommand(cmd),
            ["recording"] = cmd => voiceRecorder.HandleRecordingCommand(cmd),
            ["wisllm"] = HandleWisLlmCommand,
        };
    }

    public async Task AddCommandsIfNotExist() {
        await Log("Registering slash commands");

        var guild = client.GetGuild(Config.GuildId);
        if (guild == null)
            throw new InvalidOperationException(
                $"Configured guild '{Config.GuildId}' was not found. Ensure the bot is a member of that guild.");
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
                    .AddOption("model", ApplicationCommandOptionType.String, "Model to use (default from config)", isRequired: false)
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


    private async Task HandleWisLlmCommand(SocketSlashCommand command) {
        var sub = command.Data.Options.First().Name;
        switch (sub) {
            case "ask": await wisLlmService.HandleAskCommand(command); break;
            case "clear": await wisLlmService.HandleClearCommand(command); break;
            case "compact": await wisLlmService.HandleCompactCommand(command); break;
        }
    }

    private async Task OnSlashCommandExecuted(SocketSlashCommand command) {
        await terminal.AddLine($"[Bot] /{command.CommandName} by {command.User.Username}");

        if (commands.TryGetValue(command.CommandName, out var handler))
            await handler(command);
        else
            await terminal.AddLine($"[Bot] Unknown command: {command.CommandName}", LogLevel.Warn);
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
            await Log($"Removing guild commands for guild {Config.GuildId}");
            var guild = client.GetGuild(Config.GuildId);
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
                var guildCmds = await g.GetApplicationCommandsAsync();
                foreach (var command in guildCmds) {
                    await command.DeleteAsync();
                    await Log($"  Deleted command '{command.Name}' from guild {g.Name}");
                    await Task.Delay(100);
                }
            }

            await Log("Successfully removed all commands!");
        } catch (Exception ex) {
            await Log($"Error removing commands: {ex.Message}", LogLevel.Error);
        }
    }



    /// Terminal command: joins a hardcoded voice channel, records for 15 seconds, then stops and saves.
    public async Task TestRecord() {
        ulong testGuildId = Config.TestGuildId;
        ulong testChannelId = Config.TestVoiceChannelId;
        if (testChannelId == 0) {
            await Log("❌ TestRecord: WISBOT_TEST_VOICE_CHANNEL_ID is not set.");
            return;
        }

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

    private async Task Log(string msg, LogLevel level = LogLevel.Info)
        => await terminal.AddLine($"[Discord] {msg}", level);
}
