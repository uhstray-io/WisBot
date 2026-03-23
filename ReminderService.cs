using Discord.WebSocket;
using Microsoft.Data.Sqlite;

public record Reminder {
    public long Id { get; init; }
    public required ulong UserId { get; init; }
    public required ulong ChannelId { get; init; }
    public required string Message { get; init; }
    public required DateTime RemindAt { get; init; }
}

/// Fires one-shot reminders. Reminders are stored in the DB and survive restarts.
/// The loop checks every 30 seconds; any past-due reminders from downtime fire immediately.
public class ReminderService(Terminal terminal, DiscordSocketClient client) {
    private CancellationTokenSource? cts;

    private async Task Log(string msg) => await terminal.AddLine($"[Reminders] {msg}");

    public async Task Start() {
        int count = await GetPendingCount();
        await Log($"Started with {count} pending reminder(s)");
        cts = new CancellationTokenSource();
        _ = Task.Run(() => RunLoop(cts.Token));
    }

    public void Stop() {
        cts?.Cancel();
        cts?.Dispose();
        cts = null;
    }

    // ── Scheduling Loop ──────────────────────────────────────────────────

    private async Task RunLoop(CancellationToken token) {
        while (!token.IsCancellationRequested) {
            try {
                await CheckAndFire();
                await Task.Delay(TimeSpan.FromSeconds(30), token);
            } catch (OperationCanceledException) {
                break;
            } catch (Exception ex) {
                await Log($"Loop error: {ex.Message}");
                await Task.Delay(TimeSpan.FromSeconds(30), token);
            }
        }
    }

    private async Task CheckAndFire() {
        // Atomically claim all due reminders so a restart can't double-fire them
        var due = await ClaimDueReminders();
        foreach (var reminder in due)
            _ = Task.Run(() => Deliver(reminder));
    }

    private async Task Deliver(Reminder reminder) {
        bool sent = false;

        // Try DM first
        try {
            var user = client.GetUser(reminder.UserId);
            if (user != null) {
                var dm = await user.CreateDMChannelAsync();
                await dm.SendMessageAsync($"⏰ **Reminder:** {reminder.Message}");
                sent = true;
            }
        } catch (Exception ex) {
            await Log($"DM failed for reminder {reminder.Id}: {ex.Message}");
        }

        // Fall back to original channel with a mention
        if (!sent) {
            try {
                if (client.GetChannel(reminder.ChannelId) is ISocketMessageChannel ch) {
                    await ch.SendMessageAsync($"⏰ <@{reminder.UserId}> **Reminder:** {reminder.Message}");
                    sent = true;
                }
            } catch (Exception ex) {
                await Log($"Channel fallback failed for reminder {reminder.Id}: {ex.Message}");
            }
        }

        await Log(sent
            ? $"Delivered reminder {reminder.Id} to user {reminder.UserId}"
            : $"Could not deliver reminder {reminder.Id} — dropped");
    }

    // ── Command Handler ──────────────────────────────────────────────────

    public async Task HandleRemindCommand(SocketSlashCommand command) {
        var whenOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "when");
        var messageOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "message");

        var whenStr = (string)whenOption!.Value;
        var message = (string)messageOption!.Value;

        if (!TryParseDuration(whenStr, out var duration)) {
            await command.RespondAsync("Couldn't parse that time. Try formats like `30m`, `2h`, `1d`, or `1h30m` (max 30 days).");
            return;
        }

        await AddReminder(command.User.Id, command.Channel.Id, message, duration);

        var remindAt = DateTime.UtcNow.Add(duration);
        var formatted = FormatDuration(duration);
        await command.RespondAsync($"Got it! I'll remind you in **{formatted}** (at {remindAt:HH:mm} UTC).",
            ephemeral: true);
    }

    // ── Public API ───────────────────────────────────────────────────────

    public async Task AddReminder(ulong userId, ulong channelId, string message, TimeSpan delay) {
        var remindAt = DateTime.UtcNow.Add(delay);

        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            INSERT INTO reminders (user_id, channel_id, message, remind_at)
            VALUES ($userId, $channelId, $message, $remindAt)
            """;
        cmd.Parameters.AddWithValue("$userId", (long)userId);
        cmd.Parameters.AddWithValue("$channelId", (long)channelId);
        cmd.Parameters.AddWithValue("$message", message);
        cmd.Parameters.AddWithValue("$remindAt", remindAt.ToString("O"));

        await cmd.ExecuteNonQueryAsync();
        await Log($"Reminder set for user {userId} in {FormatDuration(delay)}");
    }

    // ── DB Queries ───────────────────────────────────────────────────────

    /// Deletes and returns all due reminders atomically via RETURNING.
    private async Task<List<Reminder>> ClaimDueReminders() {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            DELETE FROM reminders
            WHERE remind_at <= $now
            RETURNING id, user_id, channel_id, message, remind_at
            """;
        cmd.Parameters.AddWithValue("$now", DateTime.UtcNow.ToString("O"));

        List<Reminder> results = [];
        using var reader = await cmd.ExecuteReaderAsync();
        while (await reader.ReadAsync()) {
            results.Add(new Reminder {
                Id = reader.GetInt64(0),
                UserId = (ulong)reader.GetInt64(1),
                ChannelId = (ulong)reader.GetInt64(2),
                Message = reader.GetString(3),
                RemindAt = DateTime.Parse(reader.GetString(4), null, System.Globalization.DateTimeStyles.RoundtripKind),
            });
        }
        return results;
    }

    private async Task<int> GetPendingCount() {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = "SELECT COUNT(*) FROM reminders";
        return Convert.ToInt32(await cmd.ExecuteScalarAsync());
    }

    // ── Parsing & Formatting ─────────────────────────────────────────────

    /// Parses durations like "30m", "2h", "1d", "1h30m", "2h15m30s".
    public static bool TryParseDuration(string input, out TimeSpan duration) {
        duration = TimeSpan.Zero;
        ReadOnlySpan<char> span = input.AsSpan().Trim();
        if (span.IsEmpty) return false;

        TimeSpan total = TimeSpan.Zero;
        int i = 0;

        while (i < span.Length) {
            int numStart = i;
            while (i < span.Length && char.IsDigit(span[i])) i++;
            if (i == numStart) return false;
            if (!int.TryParse(span[numStart..i], out int value) || value <= 0) return false;

            int unitStart = i;
            while (i < span.Length && char.IsLetter(span[i])) i++;
            if (i == unitStart) return false;

            ReadOnlySpan<char> unit = span[unitStart..i];

            if (unit.Equals("d", StringComparison.OrdinalIgnoreCase) ||
                unit.Equals("day", StringComparison.OrdinalIgnoreCase) ||
                unit.Equals("days", StringComparison.OrdinalIgnoreCase))
                total += TimeSpan.FromDays(value);
            else if (unit.Equals("h", StringComparison.OrdinalIgnoreCase) ||
                     unit.Equals("hr", StringComparison.OrdinalIgnoreCase) ||
                     unit.Equals("hour", StringComparison.OrdinalIgnoreCase) ||
                     unit.Equals("hours", StringComparison.OrdinalIgnoreCase))
                total += TimeSpan.FromHours(value);
            else if (unit.Equals("m", StringComparison.OrdinalIgnoreCase) ||
                     unit.Equals("min", StringComparison.OrdinalIgnoreCase) ||
                     unit.Equals("minute", StringComparison.OrdinalIgnoreCase) ||
                     unit.Equals("minutes", StringComparison.OrdinalIgnoreCase))
                total += TimeSpan.FromMinutes(value);
            else if (unit.Equals("s", StringComparison.OrdinalIgnoreCase) ||
                     unit.Equals("sec", StringComparison.OrdinalIgnoreCase) ||
                     unit.Equals("second", StringComparison.OrdinalIgnoreCase) ||
                     unit.Equals("seconds", StringComparison.OrdinalIgnoreCase))
                total += TimeSpan.FromSeconds(value);
            else
                return false;
        }

        if (total <= TimeSpan.Zero || total.TotalDays > 30) return false;
        duration = total;
        return true;
    }

    /// Formats a TimeSpan as a human-readable string, e.g. "1 hour 30 minutes".
    public static string FormatDuration(TimeSpan ts) {
        List<string> parts = [];
        if (ts.Days > 0) parts.Add($"{ts.Days} {(ts.Days == 1 ? "day" : "days")}");
        if (ts.Hours > 0) parts.Add($"{ts.Hours} {(ts.Hours == 1 ? "hour" : "hours")}");
        if (ts.Minutes > 0) parts.Add($"{ts.Minutes} {(ts.Minutes == 1 ? "minute" : "minutes")}");
        if (ts.Seconds > 0 && ts.TotalMinutes < 1) parts.Add($"{ts.Seconds} {(ts.Seconds == 1 ? "second" : "seconds")}");
        return parts.Count > 0 ? string.Join(" ", parts) : "now";
    }
}
