using Discord.WebSocket;
using Microsoft.Data.Sqlite;

namespace WisBot;

public record VoiceActivityEntry {
    public long Id { get; init; }
    public required ulong UserId { get; init; }
    public required string Username { get; init; }
    public required ulong GuildId { get; init; }
    public required string GuildName { get; init; }
    public required ulong ChannelId { get; init; }
    public required string ChannelName { get; init; }
    public required string Action { get; init; }  // "joined" or "left"
    public required DateTime Timestamp { get; init; }
}

/// Passively records every voice channel join and leave event to the DB.
/// Channel hops (user moves between channels) produce a "left" for the old
/// channel and a "joined" for the new channel, preserving full history.
public class UserVoiceActivityTracker(Terminal terminal) {
    private CancellationTokenSource? retentionCts;

    private async Task Log(string msg, LogLevel level = LogLevel.Info)
        => await terminal.AddLine($"[VoiceActivity] {msg}", level);

    // ── Retention ─────────────────────────────────────────────────────────

    /// Starts the daily sweep that deletes activity rows older than
    /// Config.VoiceActivityRetentionDays. Idempotent (OnReady re-fires on reconnect).
    public void StartRetention() {
        if (retentionCts is not null) return;
        retentionCts = new CancellationTokenSource();
        _ = Task.Run(() => RunRetentionLoop(retentionCts.Token));
    }

    public void StopRetention() {
        retentionCts?.Cancel();
        retentionCts?.Dispose();
        retentionCts = null;
    }

    private async Task RunRetentionLoop(CancellationToken token) {
        while (!token.IsCancellationRequested) {
            try {
                int removed = await DeleteOldActivityAsync();
                if (removed > 0) await Log($"Retention: deleted {removed} activity row(s) older than {Config.VoiceActivityRetentionDays} days");
                await Task.Delay(TimeSpan.FromHours(12), token);
            } catch (OperationCanceledException) {
                break;
            } catch (Exception ex) {
                await Log($"Activity retention loop error: {ex.Message}", LogLevel.Error);
                await Task.Delay(TimeSpan.FromHours(12), token);
            }
        }
    }

    /// Deletes activity rows past the retention window. Timestamps are UTC "O" strings
    /// (fixed-width, lexicographically sortable — see Database.cs invariant).
    private static async Task<int> DeleteOldActivityAsync() {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();
        var cmd = conn.CreateCommand();
        cmd.CommandText = "DELETE FROM voice_activity WHERE timestamp < $cutoff";
        cmd.Parameters.AddWithValue("$cutoff", DateTime.UtcNow.AddDays(-Config.VoiceActivityRetentionDays).ToString("O"));
        return await cmd.ExecuteNonQueryAsync();
    }

    // ── Event ────────────────────────────────────────────────────────────

    public async Task OnVoiceStateUpdated(SocketUser user, SocketVoiceState before, SocketVoiceState after) {
        if (user.IsBot) return;

        if (before.VoiceChannel != null)
            await Record(user, before.VoiceChannel, "left");

        if (after.VoiceChannel != null)
            await Record(user, after.VoiceChannel, "joined");
    }

    // ── Internal ─────────────────────────────────────────────────────────

    private async Task Record(SocketUser user, SocketVoiceChannel channel, string action) {
        try {
            using var conn = new SqliteConnection(Database.ConnectionString);
            await conn.OpenAsync();

            var cmd = conn.CreateCommand();
            cmd.CommandText = """
                INSERT INTO voice_activity (user_id, username, guild_id, guild_name, channel_id, channel_name, action, timestamp)
                VALUES ($userId, $username, $guildId, $guildName, $channelId, $channelName, $action, $timestamp)
                """;
            cmd.Parameters.AddWithValue("$userId", (long)user.Id);
            cmd.Parameters.AddWithValue("$username", user.Username);
            cmd.Parameters.AddWithValue("$guildId", (long)channel.Guild.Id);
            cmd.Parameters.AddWithValue("$guildName", channel.Guild.Name);
            cmd.Parameters.AddWithValue("$channelId", (long)channel.Id);
            cmd.Parameters.AddWithValue("$channelName", channel.Name);
            cmd.Parameters.AddWithValue("$action", action);
            cmd.Parameters.AddWithValue("$timestamp", DateTime.UtcNow.ToString("O"));

            await cmd.ExecuteNonQueryAsync();
            await Log($"{user.Username} {action} #{channel.Name} in {channel.Guild.Name}");
        } catch (Exception ex) {
            await Log($"Failed to record voice activity for {user.Username}: {ex.Message}", LogLevel.Error);
        }
    }
}
