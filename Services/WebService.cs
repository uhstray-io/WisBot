using Discord;
using Discord.WebSocket;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

namespace WisBot;

/// ASP.NET Core (Kestrel) web host for the bot.
/// Today it serves GET /health for container/orchestration checks (200 once the
/// Discord gateway is connected, 503 while still starting). Bound to
/// WISBOT_HEALTH_HOST:WISBOT_HEALTH_PORT. The file-relay upload/download
/// endpoints will be added here in later Phase 8 sub-phases.
public class WebService(Terminal terminal, DiscordSocketClient client) {
    private readonly DateTime startedAt = DateTime.UtcNow;
    private WebApplication? app;

    private async Task Log(string msg, LogLevel level = LogLevel.Info)
        => await terminal.AddLine($"[Web] {msg}", level);

    public async Task Start() {
        // Kestrel uses "*" (not "+") for all interfaces; map the container value.
        string host = Config.HealthHost is "+" or "*" ? "*" : Config.HealthHost;
        var url = $"http://{host}:{Config.HealthPort}";

        var builder = WebApplication.CreateBuilder();
        builder.WebHost.UseUrls(url);
        builder.Logging.ClearProviders(); // the bot's Terminal is the log surface
        // Disable ConsoleLifetime: this web host is embedded in the bot process,
        // so it must not register its own SIGINT/SIGTERM handlers or drive process
        // shutdown — the bot (Program.cs) owns the process lifetime.
        builder.Services.AddSingleton<IHostLifetime, NoOpHostLifetime>();

        app = builder.Build();
        MapEndpoints(app);

        try {
            await app.StartAsync();
            await Log($"Web endpoint listening on {url} (/health)");
        } catch (Exception ex) {
            await Log($"Could not start web endpoint on {url}: {ex.Message}", LogLevel.Warn);
            app = null;
        }
    }

    public async Task Stop() {
        if (app is not null) await app.StopAsync();
    }

    private void MapEndpoints(WebApplication webApp) {
        webApp.MapGet("/health", () => {
            bool connected = client.ConnectionState == ConnectionState.Connected;
            var payload = new {
                status = connected ? "ok" : "starting",
                uptimeSeconds = (long)(DateTime.UtcNow - startedAt).TotalSeconds,
                latencyMs = client.Latency,
                guilds = client.Guilds.Count,
            };
            return Results.Json(payload, statusCode: connected ? 200 : 503);
        });
    }

    /// No-op host lifetime so the embedded web host doesn't register signal
    /// handlers or drive process shutdown — the bot owns the process lifetime.
    private sealed class NoOpHostLifetime : IHostLifetime {
        public Task WaitForStartAsync(CancellationToken cancellationToken) => Task.CompletedTask;
        public Task StopAsync(CancellationToken cancellationToken) => Task.CompletedTask;
    }
}
