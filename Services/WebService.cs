using Discord;
using Discord.WebSocket;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Http.Features;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using System.Net;

namespace WisBot;

/// ASP.NET Core (Kestrel) web host.
/// - GET /health — container/orchestration check (200 connected / 503 starting).
/// - GET /u/{id} — upload form (pending) or download landing (ready).
/// - POST /u/{id} — receive the file (streamed to MinIO, one per link).
/// - GET /u/{id}/file — download the stored file (forced attachment).
/// Bound to WISBOT_HEALTH_HOST:WISBOT_HEALTH_PORT.
public class WebService(Terminal terminal, DiscordSocketClient client, UploadService uploadService) {
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
        builder.WebHost.ConfigureKestrel(o => o.Limits.MaxRequestBodySize = Config.UploadMaxBytes);
        builder.Services.Configure<FormOptions>(o => o.MultipartBodyLengthLimit = Config.UploadMaxBytes);
        builder.Logging.ClearProviders(); // the bot's Terminal is the log surface
        // Disable ConsoleLifetime: this web host is embedded in the bot process,
        // so it must not register its own SIGINT/SIGTERM handlers or drive process
        // shutdown — the bot (Program.cs) owns the process lifetime.
        builder.Services.AddSingleton<IHostLifetime, NoOpHostLifetime>();

        app = builder.Build();
        MapEndpoints(app);

        try {
            await app.StartAsync();
            await Log($"Web endpoint listening on {url} (/health, /u/{{id}})");
        } catch (Exception ex) {
            // The web endpoint is essential (health checks + file relay), so a bind
            // failure is fatal — fail loudly rather than run half-up.
            await Log($"Could not start web endpoint on {url}: {ex.Message}", LogLevel.Error);
            app = null;
            throw;
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

        // File relay routes are only exposed when MinIO is configured.
        if (!Config.UploadEnabled) return;

        // Upload form (pending) or download landing (ready).
        webApp.MapGet("/u/{id}", async (string id) => {
            var rec = await uploadService.GetUploadAsync(id);
            if (rec is null) return Results.NotFound("Unknown or expired link.");
            return Results.Content(
                rec.Status == "ready" ? DownloadPage(id, rec) : UploadPage(),
                "text/html");
        });

        // Receive the file — streamed to MinIO. One file per link.
        webApp.MapPost("/u/{id}", async (string id, HttpRequest request) => {
            var rec = await uploadService.GetUploadAsync(id);
            if (rec is null) return Results.NotFound("Unknown or expired link.");
            if (rec.Status != "pending") return Results.Conflict("This link already has a file.");
            if (!request.HasFormContentType) return Results.BadRequest("Expected a file upload.");

            try {
                var form = await request.ReadFormAsync();
                var file = form.Files.GetFile("file");
                if (file is null || file.Length == 0) return Results.BadRequest("No file provided.");

                await using var stream = file.OpenReadStream();
                bool stored = await uploadService.StoreAsync(id, stream, file.FileName, file.ContentType, file.Length);
                if (!stored) return Results.Conflict("This link already has a file.");
                return Results.Redirect($"/u/{id}");
            } catch (Exception ex) when (ex is InvalidDataException or BadHttpRequestException) {
                // MultipartBodyLengthLimit / MaxRequestBodySize exceeded.
                return Results.StatusCode(StatusCodes.Status413PayloadTooLarge);
            }
        }).DisableAntiforgery();

        // Download — forced as an attachment (never inline; avoids XSS from user files).
        webApp.MapGet("/u/{id}/file", async (string id) => {
            var rec = await uploadService.GetUploadAsync(id);
            if (rec is null || rec.Status != "ready") return Results.NotFound("No file at this link.");

            return Results.Stream(
                async output => await uploadService.DownloadToAsync(id, output),
                contentType: rec.ContentType ?? "application/octet-stream",
                fileDownloadName: rec.Filename ?? id);
        });
    }

    // ── Minimal HTML (filename is HTML-encoded — it's user-supplied) ──────

    private static string UploadPage() {
        long maxMb = Config.UploadMaxBytes / (1024 * 1024);
        return $$"""
            <!doctype html><html lang="en"><head><meta charset="utf-8">
            <meta name="viewport" content="width=device-width,initial-scale=1">
            <title>WisBot — Upload</title></head>
            <body style="font-family:sans-serif;max-width:640px;margin:48px auto;padding:0 16px">
            <h2>Upload a file</h2>
            <p>Up to {{maxMb}} MB. Kept {{Config.UploadRetentionDays}} days. Once uploaded, anyone with this link can download it.</p>
            <form method="post" enctype="multipart/form-data">
              <input type="file" name="file" required>
              <button type="submit">Upload</button>
            </form>
            </body></html>
            """;
    }

    private static string DownloadPage(string id, UploadRecord rec) {
        string name = WebUtility.HtmlEncode(rec.Filename ?? "file");
        string size = rec.SizeBytes is { } b ? $"{b / (1024.0 * 1024.0):F1} MB" : "";
        return $$"""
            <!doctype html><html lang="en"><head><meta charset="utf-8">
            <meta name="viewport" content="width=device-width,initial-scale=1">
            <title>WisBot — Download</title></head>
            <body style="font-family:sans-serif;max-width:640px;margin:48px auto;padding:0 16px">
            <h2>{{name}}</h2>
            <p>{{size}}</p>
            <a href="/u/{{id}}/file"><button>Download</button></a>
            </body></html>
            """;
    }

    /// No-op host lifetime so the embedded web host doesn't register signal
    /// handlers or drive process shutdown — the bot owns the process lifetime.
    private sealed class NoOpHostLifetime : IHostLifetime {
        public Task WaitForStartAsync(CancellationToken cancellationToken) => Task.CompletedTask;
        public Task StopAsync(CancellationToken cancellationToken) => Task.CompletedTask;
    }
}
