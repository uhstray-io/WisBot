using Discord;
using Discord.WebSocket;
using Microsoft.Data.Sqlite;

namespace WisBot;

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

    private async Task Log(string msg, LogLevel level = LogLevel.Info)
        => await terminal.AddLine($"[Reminders] {msg}", level);

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
                await Log($"Loop error: {ex.Message}", LogLevel.Error);
                await Task.Delay(TimeSpan.FromSeconds(30), token);
            }
        }
    }

    // Cap concurrent deliveries so a large due-batch (e.g. after downtime) can't spawn
    // unbounded tasks or burst Discord's DM rate limit (audit L-16).
    private const int MaxConcurrentDeliveries = 8;

    private async Task CheckAndFire() {
        // Atomically claim all due reminders so a restart can't double-fire them.
        var due = await ClaimDueReminders();
        // Deliver the batch off the timer cadence with bounded concurrency, so the 30s
        // scheduler keeps ticking regardless of delivery latency.
        if (due.Count > 0)
            _ = Task.Run(() => DeliverBatch(due));
    }

    private async Task DeliverBatch(List<Reminder> due) {
        using var gate = new SemaphoreSlim(MaxConcurrentDeliveries);
        var deliveries = due.Select(async reminder => {
            await gate.WaitAsync();
            try { await Deliver(reminder); }
            finally { gate.Release(); }
        });
        await Task.WhenAll(deliveries);
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
            await Log($"DM failed for reminder {reminder.Id}: {ex.Message}", LogLevel.Error);
        }

        // Fall back to original channel with a mention
        if (!sent) {
            try {
                if (client.GetChannel(reminder.ChannelId) is ISocketMessageChannel ch) {
                    // reminder.Message is user-controlled; allow only the target user's
                    // mention to resolve so it can't ping @everyone/roles.
                    var allowed = new AllowedMentions(AllowedMentionTypes.None) { UserIds = { reminder.UserId } };
                    await ch.SendMessageAsync($"⏰ <@{reminder.UserId}> **Reminder:** {reminder.Message}",
                        allowedMentions: allowed);
                    sent = true;
                }
            } catch (Exception ex) {
                await Log($"Channel fallback failed for reminder {reminder.Id}: {ex.Message}", LogLevel.Error);
            }
        }

        if (sent) {
            await Log($"Delivered reminder {reminder.Id} to user {reminder.UserId}");
            return;
        }

        // Both paths failed (DMs closed AND channel unavailable). The row was already
        // claimed (deleted), so re-insert to retry on the next 30s pass — but give up
        // once the reminder is an hour overdue (user gone / channel deleted for good).
        if (DateTime.UtcNow - reminder.RemindAt < TimeSpan.FromHours(1)) {
            // Deliver runs fire-and-forget — a throwing requeue would otherwise vanish
            // unobserved and the already-claimed reminder would be silently lost.
            try {
                await Requeue(reminder);
                await Log($"Could not deliver reminder {reminder.Id} — re-queued for retry", LogLevel.Warn);
            } catch (Exception ex) {
                await Log($"Reminder {reminder.Id} LOST — delivery failed and requeue threw: {ex.Message}", LogLevel.Error);
            }
        } else {
            await Log($"Could not deliver reminder {reminder.Id} after 1h of retries — dropped", LogLevel.Warn);
        }
    }

    /// Re-inserts a claimed-but-undeliverable reminder, preserving its original due
    /// time so the one-hour retry window is measured from when it should have fired.
    private async Task Requeue(Reminder reminder) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            INSERT INTO reminders (user_id, channel_id, message, remind_at)
            VALUES ($userId, $channelId, $message, $remindAt)
            """;
        cmd.Parameters.AddWithValue("$userId", (long)reminder.UserId);
        cmd.Parameters.AddWithValue("$channelId", (long)reminder.ChannelId);
        cmd.Parameters.AddWithValue("$message", reminder.Message);
        cmd.Parameters.AddWithValue("$remindAt", reminder.RemindAt.ToString("O"));
        await cmd.ExecuteNonQueryAsync();
    }

    // ── Command ──────────────────────────────────────────────────────────

    public async Task HandleRemindCommand(SocketSlashCommand command) {
        var whenOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "when");
        var messageOption = command.Data.Options.FirstOrDefault(opt => opt.Name == "message");

        var whenStr = (string)whenOption!.Value;
        var message = (string)messageOption!.Value;

        if (!TryParseDuration(whenStr, out var duration)) {
            await command.RespondAsync("Couldn't parse that time. Try formats like `30m`, `2h`, `1d`, or `1h30m` (max 30 days).");
            return;
        }

        // Per-user cap on outstanding reminders (abuse / unbounded DB growth — audit L-16).
        // Enforced atomically inside the insert, so concurrent /remind calls can't race
        // past the limit.
        if (!await AddReminder(command.User.Id, command.Channel.Id, message, duration)) {
            await command.RespondAsync(
                $"You already have {Config.ReminderMaxPerUser} pending reminders (the max). " +
                "Wait for some to fire before adding more.", ephemeral: true);
            return;
        }

        var remindAt = DateTime.UtcNow.Add(duration);
        var formatted = FormatDuration(duration);
        await command.RespondAsync($"Got it! I'll remind you in **{formatted}** (at {remindAt:HH:mm} UTC).",
            ephemeral: true);
    }

    // ── Public API ───────────────────────────────────────────────────────

    /// Inserts a reminder, enforcing the per-user pending cap atomically (the count and
    /// insert are one statement under SQLite's write lock, so concurrent calls can't both
    /// slip past the limit — audit L-16). Returns false if the user is at the cap.
    public async Task<bool> AddReminder(ulong userId, ulong channelId, string message, TimeSpan delay) {
        var remindAt = DateTime.UtcNow.Add(delay);

        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            INSERT INTO reminders (user_id, channel_id, message, remind_at)
            SELECT $userId, $channelId, $message, $remindAt
            WHERE (SELECT COUNT(*) FROM reminders WHERE user_id = $userId) < $max
            """;
        cmd.Parameters.AddWithValue("$userId", (long)userId);
        cmd.Parameters.AddWithValue("$channelId", (long)channelId);
        cmd.Parameters.AddWithValue("$message", message);
        cmd.Parameters.AddWithValue("$remindAt", remindAt.ToString("O"));
        cmd.Parameters.AddWithValue("$max", Config.ReminderMaxPerUser);

        if (await cmd.ExecuteNonQueryAsync() == 0) return false; // at the per-user cap
        await Log($"Reminder set for user {userId} in {FormatDuration(delay)}");
        return true;
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

        // int.TryParse accepts values up to 2,147,483,647, so TimeSpan.FromDays/etc.
        // and the running `total +=` can overflow before the 30-day ceiling check below.
        // Treat any overflow as an unparseable (too-large) duration. (audit L-6)
        try {
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
        } catch (OverflowException) {
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
