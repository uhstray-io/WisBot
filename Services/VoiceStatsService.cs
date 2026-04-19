using Discord;
using Discord.WebSocket;
using Microsoft.Data.Sqlite;

namespace WisBot;

public record VoiceActivityRow(ulong ChannelId, string ChannelName, string Action, DateTime Timestamp);

public class VoiceStats {
    public int TotalVisits { get; init; }
    public TimeSpan TotalTime { get; init; }
    public TimeSpan LongestVisit { get; init; }
    public int PairedCount { get; init; }
    public string? FavoriteChannel { get; init; }
    public int FavoriteChannelVisits { get; init; }
    public DateTime? LastActive { get; init; }
    public int ActiveDays { get; init; }
    public DateTime? FirstActive { get; init; }
}

/// Handles the /voicestats slash command.
/// Queries the voice_activity table to compute per-user stats for the current guild.
public class VoiceStatsService(Terminal terminal) {
    private async Task Log(string msg, LogLevel level = LogLevel.Info)
        => await terminal.AddLine($"[VoiceStats] {msg}", level);

    // ── Command ──────────────────────────────────────────────────────────

    public async Task HandleCommand(SocketSlashCommand command) {
        var userOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "user");
        var target = userOption?.Value as SocketGuildUser;

        if (target == null) {
            await command.RespondAsync("Couldn't resolve that user.", ephemeral: true);
            return;
        }

        if (target.IsBot) {
            await command.RespondAsync("Bots don't have voice stats.", ephemeral: true);
            return;
        }

        ulong guildId = (command.Channel as SocketGuildChannel)!.Guild.Id;

        await command.DeferAsync();
        _ = Task.Run(async () => {
            try {
                var stats = await ComputeStats(target.Id, guildId);
                var embed = BuildEmbed(target, stats);
                await command.FollowupAsync(embed: embed);
                await Log($"Delivered voice stats for {target.Username} to {command.User.Username}");
            } catch (Exception ex) {
                await Log($"Error computing stats for {target.Username}: {ex.Message}", LogLevel.Error);
                await command.FollowupAsync("Something went wrong computing those stats.", ephemeral: true);
            }
        });
    }

    // ── Stats Computation ────────────────────────────────────────────────

    private async Task<VoiceStats> ComputeStats(ulong userId, ulong guildId) {
        var events = await LoadEvents(userId, guildId);
        if (events.Count == 0) return new VoiceStats();

        // Track open join time per channel to pair with leaves
        var openJoins = new Dictionary<ulong, DateTime>();
        var channelVisits = new Dictionary<string, int>();
        var activeDays = new HashSet<DateOnly>();
        List<TimeSpan> durations = [];
        DateTime? firstActive = null;
        DateTime? lastActive = null;

        foreach (var ev in events) {
            if (firstActive == null || ev.Timestamp < firstActive) firstActive = ev.Timestamp;
            if (ev.Timestamp > lastActive) lastActive = ev.Timestamp;

            if (ev.Action == "joined") {
                openJoins[ev.ChannelId] = ev.Timestamp;
                channelVisits[ev.ChannelName] = channelVisits.GetValueOrDefault(ev.ChannelName) + 1;
                activeDays.Add(DateOnly.FromDateTime(ev.Timestamp));
            } else if (ev.Action == "left" && openJoins.TryGetValue(ev.ChannelId, out var joinTime)) {
                var duration = ev.Timestamp - joinTime;
                if (duration > TimeSpan.Zero) durations.Add(duration);
                openJoins.Remove(ev.ChannelId);
            }
        }

        var favEntry = channelVisits.Count > 0 ? channelVisits.MaxBy(kv => kv.Value) : default;

        return new VoiceStats {
            TotalVisits = channelVisits.Values.Sum(),
            TotalTime = durations.Aggregate(TimeSpan.Zero, (acc, d) => acc + d),
            LongestVisit = durations.Count > 0 ? durations.Max() : TimeSpan.Zero,
            PairedCount = durations.Count,
            FavoriteChannel = favEntry.Key,
            FavoriteChannelVisits = favEntry.Value,
            FirstActive = firstActive,
            LastActive = lastActive,
            ActiveDays = activeDays.Count,
        };
    }

    private static async Task<List<VoiceActivityRow>> LoadEvents(ulong userId, ulong guildId) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            SELECT channel_id, channel_name, action, timestamp
            FROM voice_activity
            WHERE user_id = $userId AND guild_id = $guildId
            ORDER BY timestamp ASC
            """;
        cmd.Parameters.AddWithValue("$userId", (long)userId);
        cmd.Parameters.AddWithValue("$guildId", (long)guildId);

        List<VoiceActivityRow> rows = [];
        using var reader = await cmd.ExecuteReaderAsync();
        while (await reader.ReadAsync()) {
            rows.Add(new VoiceActivityRow(
                ChannelId: (ulong)reader.GetInt64(0),
                ChannelName: reader.GetString(1),
                Action: reader.GetString(2),
                Timestamp: DateTime.Parse(reader.GetString(3), null, System.Globalization.DateTimeStyles.RoundtripKind)
            ));
        }
        return rows;
    }

    // ── Embed Builder ────────────────────────────────────────────────────

    private static Embed BuildEmbed(SocketGuildUser user, VoiceStats stats) {
        var embed = new EmbedBuilder()
            .WithTitle($"Voice Stats — {user.DisplayName}")
            .WithThumbnailUrl(user.GetAvatarUrl() ?? user.GetDefaultAvatarUrl())
            .WithColor(new Color(0x5865F2)); // Discord blurple

        if (stats.TotalVisits == 0) {
            embed.WithDescription("No voice activity recorded for this user yet.\n*Stats are only tracked since the bot started logging.*");
            return embed.Build();
        }

        embed.AddField("Channel Visits", stats.TotalVisits.ToString(), inline: true);

        if (stats.PairedCount > 0) {
            embed.AddField("Total Hours", $"{stats.TotalTime.TotalHours:F1}h", inline: true);
            embed.AddField("Avg Visit", FormatDuration(stats.TotalTime / stats.PairedCount), inline: true);
            embed.AddField("Longest Visit", FormatDuration(stats.LongestVisit), inline: true);
        }

        if (stats.FavoriteChannel != null)
            embed.AddField("Favorite Channel", $"#{stats.FavoriteChannel} ({stats.FavoriteChannelVisits} visits)", inline: true);

        if (stats.ActiveDays > 0 && stats.FirstActive.HasValue) {
            double weeksTracked = (DateTime.UtcNow - stats.FirstActive.Value).TotalDays / 7.0;
            string avgPerWeek = weeksTracked >= 1
                ? $"{stats.ActiveDays / weeksTracked:F1} days/week"
                : "< 1 week of data";
            embed.AddField("Active Days", $"{stats.ActiveDays} days ({avgPerWeek})", inline: true);

            if (stats.PairedCount > 0)
                embed.AddField("Avg Hours/Day", $"{stats.TotalTime.TotalHours / stats.ActiveDays:F1}h", inline: true);
        }

        if (stats.LastActive.HasValue)
            embed.AddField("Last Active", FormatAgo(DateTime.UtcNow - stats.LastActive.Value), inline: true);

        embed.WithFooter("Only counts activity recorded since the bot joined the server.");
        return embed.Build();
    }

    // ── Formatting Helpers ───────────────────────────────────────────────

    private static string FormatDuration(TimeSpan ts) {
        if (ts.TotalSeconds < 60) return $"{(int)ts.TotalSeconds}s";
        if (ts.TotalMinutes < 60) return $"{(int)ts.TotalMinutes}m {ts.Seconds}s";
        if (ts.TotalHours < 24) return $"{(int)ts.TotalHours}h {ts.Minutes}m";
        return $"{(int)ts.TotalDays}d {ts.Hours}h";
    }

    private static string FormatAgo(TimeSpan ago) {
        if (ago.TotalMinutes < 1) return "just now";
        if (ago.TotalHours < 1) return $"{(int)ago.TotalMinutes}m ago";
        if (ago.TotalDays < 1) return $"{(int)ago.TotalHours}h ago";
        if (ago.TotalDays < 7) return $"{(int)ago.TotalDays}d ago";
        return ago.TotalDays < 30 ? $"{(int)(ago.TotalDays / 7)}w ago" : $"{(int)(ago.TotalDays / 30)}mo ago";
    }
}
