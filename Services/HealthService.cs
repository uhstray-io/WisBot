using Discord;
using Discord.WebSocket;
using System.Net;
using System.Text;
using System.Text.Json;

namespace WisBot;

/// Minimal HTTP health endpoint for container / orchestration health checks.
/// Listens on WISBOT_HEALTH_HOST:WISBOT_HEALTH_PORT and answers GET /health:
/// 200 once the Discord gateway is connected, 503 while still starting up.
/// Bind host defaults to "localhost" (dev); set it to "+" in the container so
/// Docker port mapping can reach it.
public class HealthService(Terminal terminal, DiscordSocketClient client) {
    private readonly DateTime startedAt = DateTime.UtcNow;
    private HttpListener? listener;

    private async Task Log(string msg, LogLevel level = LogLevel.Info)
        => await terminal.AddLine($"[Health] {msg}", level);

    public void Start() {
        var prefix = $"http://{Config.HealthHost}:{Config.HealthPort}/";
        listener = new HttpListener();
        listener.Prefixes.Add(prefix);

        try {
            listener.Start();
        } catch (Exception ex) {
            _ = Log($"Could not start health endpoint on {prefix}: {ex.Message}", LogLevel.Warn);
            listener = null;
            return;
        }

        _ = Log($"Health endpoint listening on {prefix}health");
        _ = Task.Run(AcceptLoop);
    }

    public void Stop() {
        listener?.Stop();
        listener?.Close();
        listener = null;
    }

    private async Task AcceptLoop() {
        while (listener is { IsListening: true }) {
            HttpListenerContext context;
            try {
                context = await listener.GetContextAsync();
            } catch (Exception) {
                break; // listener stopped/disposed
            }
            _ = Task.Run(() => Handle(context));
        }
    }

    private async Task Handle(HttpListenerContext context) {
        try {
            if (context.Request.Url?.AbsolutePath is not ("/health" or "/")) {
                context.Response.StatusCode = 404;
                return;
            }

            bool connected = client.ConnectionState == ConnectionState.Connected;
            var payload = new {
                status = connected ? "ok" : "starting",
                uptimeSeconds = (long)(DateTime.UtcNow - startedAt).TotalSeconds,
                latencyMs = client.Latency,
                guilds = client.Guilds.Count,
            };
            byte[] body = Encoding.UTF8.GetBytes(JsonSerializer.Serialize(payload));

            context.Response.StatusCode = connected ? 200 : 503;
            context.Response.ContentType = "application/json";
            context.Response.ContentLength64 = body.Length;
            await context.Response.OutputStream.WriteAsync(body);
        } catch {
            // Best-effort — a failed health response should never crash the bot.
        } finally {
            context.Response.Close();
        }
    }
}
