using Discord.WebSocket;
using Discord;
using Microsoft.Data.Sqlite;

/// One-shot voice presence notifications.
/// A user runs /notify targeting someone; the next time that person joins any
/// voice channel in the guild, the watcher gets a DM with a joinable channel link.
/// The notification is consumed on delivery — it does not repeat.
public class VoiceNotificationHandler(Terminal terminal, DiscordSocketClient client) {
    private async Task Log(string msg) => await terminal.AddLine($"[VoiceNotify] {msg}");

    // ── Event Handler ────────────────────────────────────────────────────

    public async Task OnVoiceStateUpdated(SocketUser user, SocketVoiceState before, SocketVoiceState after) {
        // Only fire when someone transitions from not-in-channel → in-channel
        if (user.IsBot) return;
        if (before.VoiceChannel != null || after.VoiceChannel == null) return;

        var channel = after.VoiceChannel;
        var guild = channel.Guild;

        await Log($"{user.Username} joined #{channel.Name} in {guild.Name}");

        var watchers = await ClaimWatchers(user.Id, guild.Id);
        foreach (var watcherId in watchers)
            _ = Task.Run(() => Notify(watcherId, user, channel, guild));
    }

    private async Task Notify(ulong watcherId, SocketUser target, SocketVoiceChannel channel, SocketGuild guild) {
        try {
            var watcher = client.GetUser(watcherId);
            if (watcher == null) {
                await Log($"Watcher {watcherId} not in cache — could not deliver notification");
                return;
            }

            var dm = await watcher.CreateDMChannelAsync();
            var link = $"https://discord.com/channels/{guild.Id}/{channel.Id}";

            await dm.SendMessageAsync(
                $"📢 **{target.Username}** just joined **{guild.Name}**'s voice channel **{channel.Name}**!\n" +
                $"[Click here to join]({link})");

            await Log($"Notified {watcher.Username} that {target.Username} joined #{channel.Name}");
        } catch (Exception ex) {
            await Log($"Failed to notify watcher {watcherId}: {ex.Message}");
        }
    }

    // ── Command Handler ──────────────────────────────────────────────────

    public async Task HandleNotifyCommand(SocketSlashCommand command) {
        var userOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "user");
        var target = userOption!.Value as SocketGuildUser;

        if (target == null) {
            await command.RespondAsync("Couldn't resolve that user.", ephemeral: true);
            return;
        }

        if (target.Id == command.User.Id) {
            await command.RespondAsync("You can't set a notification for yourself.", ephemeral: true);
            return;
        }

        if (target.IsBot) {
            await command.RespondAsync("Bots don't count.", ephemeral: true);
            return;
        }

        ulong guildId = (command.Channel as SocketGuildChannel)!.Guild.Id;
        await AddNotification(command.User.Id, target.Id, guildId);
        await command.RespondAsync(
            $"Got it! I'll DM you the next time **{target.DisplayName}** joins a voice channel.",
            ephemeral: true);
    }

    // ── Public API ───────────────────────────────────────────────────────

    public async Task AddNotification(ulong watcherId, ulong targetId, ulong guildId) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            INSERT INTO voice_notifications (watcher_id, target_id, guild_id, is_active)
            VALUES ($watcher, $target, $guild, 1)
            ON CONFLICT (watcher_id, target_id, guild_id) DO UPDATE SET is_active = 1
            """;
        cmd.Parameters.AddWithValue("$watcher", (long)watcherId);
        cmd.Parameters.AddWithValue("$target", (long)targetId);
        cmd.Parameters.AddWithValue("$guild", (long)guildId);

        await cmd.ExecuteNonQueryAsync();
    }

    // ── DB Queries ───────────────────────────────────────────────────────

    /// Atomically claims all active watchers for a target joining in a guild
    /// by flipping is_active to 0, preventing double-delivery on restart.
    private async Task<List<ulong>> ClaimWatchers(ulong targetId, ulong guildId) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            UPDATE voice_notifications
            SET is_active = 0
            WHERE target_id = $target AND guild_id = $guild AND is_active = 1
            RETURNING watcher_id
            """;
        cmd.Parameters.AddWithValue("$target", (long)targetId);
        cmd.Parameters.AddWithValue("$guild", (long)guildId);

        List<ulong> watchers = [];
        using var reader = await cmd.ExecuteReaderAsync();
        while (await reader.ReadAsync())
            watchers.Add((ulong)reader.GetInt64(0));

        return watchers;
    }
}
