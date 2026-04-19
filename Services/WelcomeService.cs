using Discord.WebSocket;
using Microsoft.Data.Sqlite;

namespace WisBot;

/// Sends a randomized welcome message the first time a user joins a guild.
/// Tracks welcomed users in the welcomed_users table (see Database.cs).
public class WelcomeService(Terminal terminal) {
    private static readonly string[] Messages = [
        "👋 Welcome to **{guild}**, {mention}! Glad to have you here.",
        "🎉 {mention} just landed in **{guild}**! Welcome aboard!",
        "🚀 A new challenger approaches! Welcome, {mention}!",
        "🌟 **{guild}** just got better — {mention} has arrived!",
        "🎊 Everyone say hi to {mention}! Welcome to the crew!",
        "🛸 {mention} has entered the server. We come in peace... probably.",
        "🔔 Heads up, **{guild}**! {mention} just walked through the door.",
        "🎮 Player {mention} has joined the game. Welcome!",
        "🍕 {mention} just joined **{guild}**. Hope you brought snacks.",
        "⚡ {mention} has connected to **{guild}**!",
        "🧭 {mention} found their way to **{guild}**. Welcome, traveler!",
        "🦉 A wild {mention} appeared in **{guild}**!",
        "🎸 The gang's all here now that {mention} showed up. Welcome!",
        "🌊 {mention} just dove into **{guild}**. Hope the water's warm!",
        "🏰 The gates of **{guild}** open wide for {mention}. Welcome!",
    ];

    private async Task Log(string msg, LogLevel level = LogLevel.Info)
        => await terminal.AddLine($"[Welcome] {msg}", level);

    // ── Event ────────────────────────────────────────────────────────────

    public async Task OnUserJoined(SocketGuildUser user) {
        await Log($"User joined: {user.Username} ({user.Id}) in {user.Guild.Name}");

        // Atomically insert and check — if 0 rows affected, user was already welcomed.
        // Two-step check-then-insert would be a TOCTOU race: two concurrent events could
        // both pass the check before either writes, causing a double welcome message.
        if (!await RecordFirstJoin(user.Guild.Id, user.Id)) {
            await Log($"{user.Username} has joined before — skipping welcome.");
            return;
        }

        var channel = (ISocketMessageChannel?)user.Guild.SystemChannel
            ?? user.Guild.TextChannels.OrderBy(c => c.Position).FirstOrDefault();

        if (channel == null) {
            await Log($"No channel found to welcome {user.Username}");
            return;
        }

        var message = Messages[Random.Shared.Next(Messages.Length)]
            .Replace("{guild}", user.Guild.Name)
            .Replace("{mention}", user.Mention);

        await channel.SendMessageAsync(message);

        string channelName = channel is SocketGuildChannel gc ? gc.Name : "unknown";
        await Log($"Welcomed {user.Username} in #{channelName}");
    }

    // ── DB Queries ───────────────────────────────────────────────────────

    /// Inserts the user as welcomed. Returns true if this is their first join, false if already recorded.
    /// Atomic: INSERT OR IGNORE + check rows affected eliminates the check-then-insert race.
    private async Task<bool> RecordFirstJoin(ulong guildId, ulong userId) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            INSERT OR IGNORE INTO welcomed_users (guild_id, user_id)
            VALUES ($guild, $user)
            """;
        cmd.Parameters.AddWithValue("$guild", (long)guildId);
        cmd.Parameters.AddWithValue("$user", (long)userId);

        return await cmd.ExecuteNonQueryAsync() > 0;
    }
}
