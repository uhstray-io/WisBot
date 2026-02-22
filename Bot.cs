using Discord;
using Discord.WebSocket;

public class Bot {
    private readonly string _wiswardId = "265656652664012800";
    private readonly string _wisbotId = "WisBot#2348";
    private readonly ulong _uhstrayGuildId = 889910011113906186;

    private string _token = null!;

    private DiscordSocketClient _client = null!;

    public Terminal Terminal { get; }
    private VoiceRecorder _voiceRecorder = null!;

    public Bot(Terminal terminal) {
        Terminal = terminal;
        _voiceRecorder = new VoiceRecorder(terminal);
    }

    public async Task StartBot() {
        var config = new DiscordSocketConfig {
            GatewayIntents =
                GatewayIntents.AllUnprivileged
                | GatewayIntents.MessageContent
                | GatewayIntents.GuildMessages,
        };

        _client = new DiscordSocketClient(config);
        _client.Log += OnLog;
        _client.MessageUpdated += OnMessageUpdated;
        _client.MessageReceived += OnMessageReceived;
        _client.Ready += OnReady;
        _client.UserIsTyping += OnUserIsTyping;
        _client.SlashCommandExecuted += OnSlashCommandExecuted;

        await InitBot();
        await _client.LoginAsync(TokenType.Bot, _token);
        await _client.StartAsync();
    }

    private async Task InitBot() {
        await Log("Reading discord key from file...");
        var content = await File.ReadAllTextAsync("discord.key");
        _token = content.Trim();
    }



    private async Task OnUserIsTyping(Cacheable<IUser, ulong> _user, Cacheable<IMessageChannel, ulong> _channel) {
        var user = await _user.GetOrDownloadAsync();
        var channel = await _channel.GetOrDownloadAsync();
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

        await AddCommandsIfNotExist();
    }

    public async Task AddCommandsIfNotExist() {
        await Log("Registering slash commands");

        var guild = _client.GetGuild(_uhstrayGuildId);
        var existingGuildCommands = await guild.GetApplicationCommandsAsync();

        // Uhstray guild commands
        // if (!existingGuildCommands.Any(cmd => cmd.Name == "eatmehnuts")) {
        //     var command = new SlashCommandBuilder()
        //         .WithName("eatmehnuts")
        //         .WithDescription("Ask the bot if it wants to eat deez nuts.")
        //         .Build();

        //     await guild.CreateApplicationCommandAsync(command);
        //     await Log($"Registered guild slash command: /{command.Name} in '{guild.Name}'");
        // }


        // if (!existingGuildCommands.Any(cmd => cmd.Name == "favoritecolor")) {
        //     var command = new SlashCommandBuilder()
        //         .WithName("favoritecolor")
        //         .WithDescription("Set your favorite color")
        //         .AddOption(new SlashCommandOptionBuilder()
        //             .WithName("color")
        //             .WithDescription("Choose a color")
        //             .WithRequired(true)
        //             .WithType(ApplicationCommandOptionType.String)
        //             .AddChoice("Red", "red")
        //             .AddChoice("Blue", "blue")
        //             .AddChoice("Green", "green")
        //             .AddChoice("Yellow", "yellow")
        //         )
        //         .Build();

        //     await guild.CreateApplicationCommandAsync(command);
        //     await Log($"Registered guild slash command: /{command.Name} in '{guild.Name}'");
        // }

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




        // Global commands
        var existingGlobalCommands = await _client.GetGlobalApplicationCommandsAsync();
        // if (!existingGlobalCommands.Any(cmd => cmd.Name == "eatdeeznuts")) {
        //     var command = new SlashCommandBuilder()
        //         .WithName("eatdeeznuts")
        //         .WithDescription("Ask the bot if it wants to eat deez nuts.")
        //         .Build();

        //     await _client.CreateGlobalApplicationCommandAsync(command);
        //     await Log($"Registered global slash command: /{command.Name}");
        // }

        if (!existingGlobalCommands.Any(cmd => cmd.Name == "wisllm")) {
            var command = new SlashCommandBuilder()
                .WithName("wisllm")
                .WithDescription("Replies with Nothing atm.")
                .AddOption(new SlashCommandOptionBuilder()
                    .WithName("prompt")
                    .WithDescription("The prompt to send to the LLM")
                    .WithRequired(true)
                    .WithType(ApplicationCommandOptionType.String)
                )
                .Build();

            await _client.CreateGlobalApplicationCommandAsync(command);
            await Log($"Registered global slash command: /{command.Name}");
        }
    }


    private async Task OnSlashCommandExecuted(SocketSlashCommand command) {
        // Log User and command info
        await Log($"Slash command executed: {command.CommandName} by {command.User.Username}");
        await Log($"Command options: {string.Join(", ", command.Data.Options.Select(opt => $"{opt.Name}={opt.Value}"))}");


        // if (command.CommandName == "eatdeeznuts") {
        //     if (Random.Shared.NextDouble() < 0.1) {
        //         await command.RespondAsync("Sure, I could go for some nuts.");
        //         return;
        //     }
        //     await command.RespondAsync("No thanks, I'm good.");
        //     return;
        // }

        // if (command.CommandName == "eatmehnuts") {
        //     if (Random.Shared.NextDouble() < 0.1) {
        //         await command.RespondAsync("Aye Aye Captain!");
        //         return;
        //     }
        //     await command.RespondAsync("No thanks, I'm good. Maybe next time :)");
        //     return;
        // }

        // if (command.CommandName == "favoritecolor") {
        //     var colorOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "color");
        //     if (colorOption != null) {
        //         var colorValue = (string)colorOption.Value!;
        //         await command.RespondAsync($"Your favorite color is set to {colorValue}! jk i dont really care!");
        //     } else {
        //         await command.RespondAsync("You didn't provide a color!");
        //     }
        //     return;
        // }

        if (command.CommandName == "wisllm") {
            await command.RespondAsync("Nothing to see here yet!");
            command.Data.Options.ToList().ForEach(async opt => {
                await Log($"Option: {opt.Name} = {opt.Value}");
            });
            return;
        }

        if (command.CommandName == "recording") {
            var actionOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "action");
            var sendFileOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "sendfile");
            var mergeAudioOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "mergeaudio");

            if (actionOption == null) {
                await command.RespondAsync("❌ Action parameter is required!");
                return;
            }

            var action = (string)actionOption.Value!;
            var sendFile = sendFileOption != null && (bool)sendFileOption.Value!;
            var mergeAudio = mergeAudioOption != null && (bool)mergeAudioOption.Value!;

            _ = Log($"Recording command received with action: {action}, sendfile: {sendFile}, mergeaudio: {mergeAudio}");

            if (action == "start") {
                await StartRecordingInVoiceChannel(command);
                return;
            }

            if (action == "stop") {
                await StopRecordingAndProcess(command, sendFile, mergeAudio);
                return;
            }

            await command.RespondAsync("❌ Invalid action. Use 'start' or 'stop'.");
            return;
        }

        async Task StartRecordingInVoiceChannel(SocketSlashCommand command) {
            // Get the user who executed the command
            var user = command.User as SocketGuildUser;

            if (user?.VoiceChannel == null) {
                await command.RespondAsync("You must be in a voice channel to use this command!");
                return;
            }

            // Respond immediately to avoid 3-second timeout
            await command.RespondAsync("Joining voice channel and starting recording...");

            // Do the long-running work in the background
            _ = Task.Run(async () => {
                try {
                    var result = await _voiceRecorder.JoinAndRecordUser(user);

                    await Log($"Finished recording for {user.Username}. Result: {result}");
                    await command.FollowupAsync(result);
                } catch (Exception ex) {
                    await Log($"Error in recording task: {ex.Message}");
                    await command.FollowupAsync($"Error starting recording: {ex.Message}");
                }
            });
        }

        async Task StopRecordingAndProcess(SocketSlashCommand command, bool sendFile, bool mergeAudio) {
            await command.RespondAsync("Stopping recording and processing audio files...");

            // Run the long-running stop/save operation in the background
            _ = Task.Run(async () => {
                try {
                    var result = await _voiceRecorder.StopRecordingAndSave(command.Channel, sendFiles: sendFile, mergeAudio: mergeAudio);
                    await Log($"Finished processing audio files. Saved {result.Count} file(s).");

                    if (result.Count > 0) {
                        if (sendFile) {
                            if (mergeAudio) {
                                await command.FollowupAsync($"✅ Recording saved and sent! Merged {result.Count} user(s) into 1 file.");
                            } else {
                                await command.FollowupAsync($"✅ Recording saved and sent! Processed {result.Count} user(s).");
                            }
                        } else {
                            if (mergeAudio) {
                                await command.FollowupAsync($"✅ Recording saved locally! Merged {result.Count} user(s) into 1 file. Use a file link service to share (coming soon).");
                            } else {
                                await command.FollowupAsync($"✅ Recording saved locally! Processed {result.Count} user(s). Use a file link service to share (coming soon).");
                            }
                        }
                    } else {
                        await command.FollowupAsync("⚠️ No audio was captured during the recording session.");
                    }
                } catch (Exception ex) {
                    await Log($"Error processing recording: {ex.Message}");
                    await command.FollowupAsync($"❌ Error processing recording: {ex.Message}");
                }
            });
        }
    }

    public async Task RemoveAllCommands() {
        await Log("Removing all existing commands...");

        try {
            // Remove global commands
            await Log("Removing global commands");
            var globalCommands = await _client.GetGlobalApplicationCommandsAsync();
            foreach (var command in globalCommands) {
                await command.DeleteAsync();
                await Log($"  Deleted global command: {command.Name}");
                await Task.Delay(100);
            }

            // Remove guild-specific commands for the configured guild
            await Log($"Removing guild commands for guild {_uhstrayGuildId}");
            var guild = _client.GetGuild(_uhstrayGuildId);
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
            foreach (var g in _client.Guilds) {
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

        var guild = _client.GetGuild(testGuildId);
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
        var joinResult = await _voiceRecorder.JoinAndRecordChannel(channel);
        await Log($"🎙️ TestRecord: {joinResult}");

        await Log("🎙️ TestRecord: Recording for 15 seconds...");
        await Task.Delay(TimeSpan.FromSeconds(15));

        await Log("🎙️ TestRecord: Stopping recording...");
        var files = await _voiceRecorder.StopRecordingAndSave();

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
        await Terminal.AddLine($"[Discord] {msg}");
    }
}