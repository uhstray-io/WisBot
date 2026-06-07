using Discord;
using Discord.WebSocket;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Http.Features;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.AspNetCore.RateLimiting;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using System.Net;
using System.Threading.RateLimiting;

namespace WisBot;

/// ASP.NET Core (Kestrel) web host.
/// - GET /health — container/orchestration check (200 connected / 503 starting).
/// - GET /u/{id} — upload form (pending) or download landing (ready).
/// - POST /u/{id} — receive the file (streamed to MinIO, one per link).
/// - GET /u/{id}/file — download the stored file (forced attachment).
/// Bound to WISBOT_HEALTH_HOST:WISBOT_HEALTH_PORT.
public class WebService(Terminal terminal, DiscordSocketClient client, UploadService uploadService) {
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

        // Per-client fixed-window limit on the public upload routes (the only
        // internet-facing surface). Partition by the Caddy-forwarded client IP.
        builder.Services.AddRateLimiter(options => {
            options.RejectionStatusCode = StatusCodes.Status429TooManyRequests;
            options.AddPolicy("uploads", httpContext => {
                string key = httpContext.Request.Headers["X-Forwarded-For"].FirstOrDefault()?.Split(',')[0].Trim()
                             ?? httpContext.Connection.RemoteIpAddress?.ToString()
                             ?? "unknown";
                return RateLimitPartition.GetFixedWindowLimiter(key, _ => new FixedWindowRateLimiterOptions {
                    PermitLimit = Config.UploadRateLimitPerMinute,
                    Window = TimeSpan.FromMinutes(1),
                    QueueLimit = 0,
                });
            });
        });

        app = builder.Build();
        app.UseRateLimiter();
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

    /// Active-content types that must never be reflected as the download Content-Type —
    /// Content-Disposition: attachment is the primary XSS defense, this is the second layer.
    private static readonly string[] UnsafeContentTypes =
        ["text/html", "application/xhtml+xml", "image/svg+xml", "text/xml", "application/xml",
         "text/javascript", "application/javascript", "application/ecmascript"];

    private static string SafeContentType(string? stored) {
        if (string.IsNullOrWhiteSpace(stored)
            || !System.Net.Http.Headers.MediaTypeHeaderValue.TryParse(stored, out var parsed)
            || parsed.MediaType is null
            || UnsafeContentTypes.Contains(parsed.MediaType, StringComparer.OrdinalIgnoreCase))
            return "application/octet-stream";
        return parsed.MediaType; // type/subtype only — drop client-supplied parameters
    }

    private void MapEndpoints(WebApplication webApp) {
        // Defense-in-depth for user-supplied file content: never let browsers sniff types.
        webApp.Use(async (ctx, next) => {
            ctx.Response.Headers["X-Content-Type-Options"] = "nosniff";
            await next();
        });

        // Public payload is status-only: this endpoint can be internet-reachable, and
        // uptime/latency/guild-count are operator detail (use /status in Discord).
        webApp.MapGet("/health", () => {
            bool connected = client.ConnectionState == ConnectionState.Connected;
            return Results.Json(new { status = connected ? "ok" : "starting" },
                statusCode: connected ? 200 : 503);
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
        }).RequireRateLimiting("uploads");

        // Receive the file — streamed to MinIO. One file per link.
        webApp.MapPost("/u/{id}", async (string id, HttpRequest request) => {
            var rec = await uploadService.GetUploadAsync(id);
            if (rec is null) return Results.NotFound("Unknown or expired link.");
            if (rec.Status != "pending") return Results.Conflict("This link already has a file.");
            if (!request.HasFormContentType) return Results.BadRequest("Expected a file upload.");

            // Claim the single-use link BEFORE buffering the body — a non-claimable
            // request (replay / lost race) is rejected without buffering the upload.
            if (!await uploadService.TryClaimForUploadAsync(id))
                return Results.Conflict("This link already has a file.");

            bool stored = false;
            try {
                var form = await request.ReadFormAsync();
                var file = form.Files.GetFile("file");
                if (file is null || file.Length == 0) return Results.BadRequest("No file provided.");

                await using var stream = file.OpenReadStream();
                await uploadService.StoreClaimedAsync(id, stream, file.FileName, file.ContentType, file.Length);
                stored = true;
                return Results.Redirect($"/u/{id}");
            } catch (Exception ex) when (ex is InvalidDataException or BadHttpRequestException) {
                // MultipartBodyLengthLimit / MaxRequestBodySize exceeded.
                return Results.StatusCode(StatusCodes.Status413PayloadTooLarge);
            } finally {
                // Any non-success path (no file, oversized, client disconnect) frees the
                // link for retry. RevertToPending is idempotent, so a double-release
                // (StoreClaimedAsync also reverts on its own failure) is harmless.
                if (!stored) await uploadService.ReleaseClaimAsync(id);
            }
        }).DisableAntiforgery().RequireRateLimiting("uploads");

        // Download — forced as an attachment (never inline; avoids XSS from user files).
        webApp.MapGet("/u/{id}/file", async (string id) => {
            var rec = await uploadService.GetUploadAsync(id);
            if (rec is null || rec.Status != "ready") return Results.NotFound("No file at this link.");

            // Probe before committing a 200 — once streaming starts the headers are
            // sent, and a missing object would yield a truncated 'successful' download.
            bool? exists = await uploadService.ObjectExistsAsync(id);
            if (exists == false) return Results.NotFound("The file behind this link is no longer available.");
            if (exists is null) return Results.StatusCode(StatusCodes.Status503ServiceUnavailable);

            return Results.Stream(
                async output => {
                    try {
                        await uploadService.DownloadToAsync(id, output);
                    } catch (Exception ex) {
                        // Headers are committed; surface the otherwise-silent partial download.
                        await Log($"Download stream aborted mid-transfer: {ex.Message}", LogLevel.Error);
                        throw;
                    }
                },
                contentType: SafeContentType(rec.ContentType),
                fileDownloadName: rec.Filename ?? id);
        }).RequireRateLimiting("uploads");
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
