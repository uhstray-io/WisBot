using Discord;
using Discord.WebSocket;
using System.Diagnostics;

namespace WisBot;

/// Handles the /status slash command.
/// Returns a monitoring snapshot of the bot process — no DB or historical data required.
public class StatusService(Terminal terminal, DiscordSocketClient client) {
    private async Task Log(string msg, LogLevel level = LogLevel.Info)
        => await terminal.AddLine($"[Status] {msg}", level);

    public async Task HandleCommand(SocketSlashCommand command) {
        var embed = BuildEmbed();
        await command.RespondAsync(embed: embed);
        await Log($"Status requested by {command.User.Username}");
    }

    private Embed BuildEmbed() {
        var proc = Process.GetCurrentProcess();
        var uptime = DateTime.UtcNow - proc.StartTime.ToUniversalTime();

        double workingSetMb = proc.WorkingSet64 / 1024.0 / 1024.0;
        double gcHeapMb = GC.GetTotalMemory(false) / 1024.0 / 1024.0;
        double totalAllocatedMb = GC.GetTotalAllocatedBytes(precise: false) / 1024.0 / 1024.0;
        double cpuPercent = uptime.TotalMilliseconds > 0
            ? proc.TotalProcessorTime.TotalMilliseconds / (uptime.TotalMilliseconds * Environment.ProcessorCount) * 100
            : 0;

        ThreadPool.GetAvailableThreads(out int availableWorkers, out int availableIo);
        ThreadPool.GetMaxThreads(out int maxWorkers, out int maxIo);

        int cachedUsers = client.Guilds.Sum(g => g.Users.Count);
        return new EmbedBuilder()
            .WithTitle("WisBot Status")
            .WithColor(Color.Green)
            .WithTimestamp(DateTimeOffset.UtcNow)
            .AddField("Uptime", FormatUptime(uptime), inline: true)
            .AddField("WS Latency", $"{client.Latency} ms", inline: true)
            .AddField("CPU (avg)", $"{cpuPercent:F1}%", inline: true)
            .AddField("Memory (RSS)", $"{workingSetMb:F1} MB", inline: true)
            .AddField("GC Heap", $"{gcHeapMb:F1} MB", inline: true)
            .AddField("Total Allocated", $"{totalAllocatedMb:F1} MB", inline: true)
            .AddField("Handles", proc.HandleCount.ToString(), inline: true)
            .AddField("Threads", proc.Threads.Count.ToString(), inline: true)
            .AddField("Thread Pool", $"{availableWorkers}/{maxWorkers} workers · {availableIo}/{maxIo} I/O", inline: true)
            .AddField("GC Gen 0/1/2", $"{GC.CollectionCount(0)} / {GC.CollectionCount(1)} / {GC.CollectionCount(2)}", inline: true)
            .AddField("Guilds", client.Guilds.Count.ToString(), inline: true)
            .AddField("Cached Users", cachedUsers.ToString(), inline: true)
            .AddField(".NET Runtime", $"v{Environment.Version}", inline: true)
            .AddField("Platform", Environment.OSVersion.ToString(), inline: true)
            .AddField("PID", proc.Id.ToString(), inline: true)
            .WithFooter("CPU % is averaged across full uptime · Thread Pool shows available/max slots")
            .Build();
    }

    private static string FormatUptime(TimeSpan ts) {
        if (ts.TotalSeconds < 60) return $"{(int)ts.TotalSeconds}s";
        if (ts.TotalMinutes < 60) return $"{(int)ts.TotalMinutes}m {ts.Seconds}s";
        if (ts.TotalHours < 24) return $"{(int)ts.TotalHours}h {ts.Minutes}m";
        return $"{(int)ts.TotalDays}d {ts.Hours}h {ts.Minutes}m";
    }
}
