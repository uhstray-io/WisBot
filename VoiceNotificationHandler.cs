using Discord.WebSocket;
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

    // ── Public API ───────────────────────────────────────────────────────

    public async Task AddNotification(ulong watcherId, ulong targetId, ulong guildId) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        // INSERT OR REPLACE: enforced by UNIQUE(watcher_id, target_id, guild_id)
        cmd.CommandText = """
            INSERT OR REPLACE INTO voice_notifications (watcher_id, target_id, guild_id)
            VALUES ($watcher, $target, $guild)
            """;
        cmd.Parameters.AddWithValue("$watcher", (long)watcherId);
        cmd.Parameters.AddWithValue("$target", (long)targetId);
        cmd.Parameters.AddWithValue("$guild", (long)guildId);

        await cmd.ExecuteNonQueryAsync();
    }

    // ── DB Queries ───────────────────────────────────────────────────────

    /// Atomically claims and removes all watchers for a target joining in a guild.
    private async Task<List<ulong>> ClaimWatchers(ulong targetId, ulong guildId) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            DELETE FROM voice_notifications
            WHERE target_id = $target AND guild_id = $guild
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
