namespace WisBot;

/// Loads configuration at startup. Resolution order per key: process environment
/// variable (how the container receives config via compose `env_file`), then the
/// local `.env` file (dev convenience), then a safe default. The Discord token may
/// also come from a `discord.key` file (local dev). No site-specific values are
/// hardcoded — guild/channel IDs and paths are all configurable.
public static class Config {
    public static string DiscordToken { get; private set; } = string.Empty;
    public static string OllamaEndpoint { get; private set; } = "http://localhost:11434";
    public static string OllamaDefaultModel { get; private set; } = "llama3";
    public static int WisLlmContextLimit { get; private set; } = 10;
    public static int WisLlmWarnAtPercent { get; private set; } = 75;
    public static int WisLlmContextSize { get; private set; } = 0;

    // Discord guild this bot serves — slash command registration target. Required.
    public static ulong GuildId { get; private set; }
    // /testrecord terminal command targets. TestGuildId defaults to GuildId when unset.
    public static ulong TestGuildId { get; private set; }
    public static ulong TestVoiceChannelId { get; private set; }

    // Persistence — point these at mounted volumes in the container.
    public static string DbPath { get; private set; } = "wisbot.db";
    public static string RecordingsDir { get; private set; } = "recordings";

    // Voice recording safety caps. Audio is buffered in RAM during capture
    // (~11 MB/min/user), so an uncapped session can OOM the process — capture
    // auto-stops when EITHER the wall-clock or the aggregate-byte ceiling is hit.
    public static int RecordingMaxMinutes { get; private set; } = 120;
    public static long RecordingMaxBytes { get; private set; } = 2L * 1024 * 1024 * 1024;

    // HTTP health endpoint. Host defaults to "localhost" (dev); set "+" in the
    // container so Docker port mapping can reach it.
    public static string HealthHost { get; private set; } = "localhost";
    public static int HealthPort { get; private set; } = 8080;

    // File relay (Phase 8) — public base URL used to build upload/download links,
    // plus the per-file size cap and retention window.
    public static string PublicBaseUrl { get; private set; } = "http://localhost:8080";
    // Per-file cap (default 100 MB — well above Discord's limit, the relay's purpose;
    // operators raise it via env). Lower default reduces the buffered-upload DoS surface.
    public static long UploadMaxBytes { get; private set; } = 100L * 1024 * 1024;
    public static int UploadRetentionDays { get; private set; } = 30;
    // Per-user abuse caps on /upload (storage-exhaustion guard) and a public-endpoint
    // request rate limit (per client IP, fixed window).
    public static int UploadMaxLinksPerUser { get; private set; } = 20;
    public static long UploadMaxBytesPerUser { get; private set; } = 2L * 1024 * 1024 * 1024;
    public static int UploadRateLimitPerMinute { get; private set; } = 30;

    // MinIO object storage for the file relay. An empty endpoint disables uploads.
    public static string MinioEndpoint { get; private set; } = "";
    public static string MinioAccessKey { get; private set; } = "";
    public static string MinioSecretKey { get; private set; } = "";
    public static string MinioBucket { get; private set; } = "wisbot-uploads";
    public static bool MinioUseSsl { get; private set; } = false;
    public static bool UploadEnabled => !string.IsNullOrWhiteSpace(MinioEndpoint);

    public static void Load(string envPath = ".env", string tokenPath = "discord.key") {
        // discord.key is the local dev path; env var / .env is the Docker/CI path
        if (File.Exists(tokenPath))
            DiscordToken = File.ReadAllText(tokenPath).Trim();

        // Parse the local .env file (dev) into a lookup; process env vars override it (container).
        Dictionary<string, string> fileVars = [];
        if (File.Exists(envPath)) {
            foreach (var raw in File.ReadAllLines(envPath)) {
                var line = raw.Trim();
                if (line.StartsWith('#') || !line.Contains('=')) continue;

                int idx = line.IndexOf('=');
                fileVars[line[..idx].Trim()] = line[(idx + 1)..].Trim();
            }
        }

        // Resolve a key from the environment first, then the .env file. Blank → treated as absent.
        string? Get(string key) {
            var value = Environment.GetEnvironmentVariable(key);
            if (string.IsNullOrWhiteSpace(value)) fileVars.TryGetValue(key, out value);
            return string.IsNullOrWhiteSpace(value) ? null : value.Trim();
        }

        // Deployment workflow appends DISCORD_TOKEN_WISBOT; env var or .env both work.
        if (Get("DISCORD_TOKEN_WISBOT") is { } token) DiscordToken = token;

        if (Get("OLLAMA_ENDPOINT") is { } endpoint) OllamaEndpoint = endpoint;
        if (Get("OLLAMA_DEFAULT_MODEL") is { } model) OllamaDefaultModel = model;

        if (Get("WISLLM_CONTEXT_LIMIT") is { } limitStr && int.TryParse(limitStr, out int limit) && limit > 0)
            WisLlmContextLimit = limit;
        if (Get("WISLLM_WARN_AT_PERCENT") is { } pctStr && int.TryParse(pctStr, out int pct) && pct is > 0 and <= 100)
            WisLlmWarnAtPercent = pct;
        if (Get("WISLLM_CONTEXT_SIZE") is { } sizeStr && int.TryParse(sizeStr, out int size) && size > 0)
            WisLlmContextSize = size;

        if (Get("WISBOT_GUILD_ID") is { } guildStr && ulong.TryParse(guildStr, out ulong guildId))
            GuildId = guildId;

        // Test targets default to the main guild; override only if explicitly set.
        TestGuildId = GuildId;
        if (Get("WISBOT_TEST_GUILD_ID") is { } testGuildStr && ulong.TryParse(testGuildStr, out ulong testGuildId))
            TestGuildId = testGuildId;
        if (Get("WISBOT_TEST_VOICE_CHANNEL_ID") is { } testChanStr && ulong.TryParse(testChanStr, out ulong testChanId))
            TestVoiceChannelId = testChanId;

        if (Get("WISBOT_DB_PATH") is { } dbPath) DbPath = dbPath;
        if (Get("WISBOT_RECORDINGS_DIR") is { } recordingsDir) RecordingsDir = recordingsDir;

        if (Get("WISBOT_RECORDING_MAX_MINUTES") is { } recMinStr && int.TryParse(recMinStr, out int recMin) && recMin > 0)
            RecordingMaxMinutes = recMin;
        if (Get("WISBOT_RECORDING_MAX_BYTES") is { } recByteStr && long.TryParse(recByteStr, out long recBytes) && recBytes > 0)
            RecordingMaxBytes = recBytes;

        if (Get("WISBOT_HEALTH_HOST") is { } healthHost) HealthHost = healthHost;
        if (Get("WISBOT_HEALTH_PORT") is { } healthPortStr && int.TryParse(healthPortStr, out int healthPort) && healthPort is > 0 and <= 65535)
            HealthPort = healthPort;

        if (Get("WISBOT_PUBLIC_BASE_URL") is { } baseUrl) {
            if (!Uri.TryCreate(baseUrl, UriKind.Absolute, out Uri? parsed) ||
                (parsed.Scheme != Uri.UriSchemeHttp && parsed.Scheme != Uri.UriSchemeHttps))
                throw new InvalidOperationException(
                    "WISBOT_PUBLIC_BASE_URL must be an absolute http(s) URL (e.g. https://up.example.io).");
            PublicBaseUrl = baseUrl.TrimEnd('/');
        }
        if (Get("WISBOT_UPLOAD_MAX_BYTES") is { } maxStr && long.TryParse(maxStr, out long maxBytes) && maxBytes > 0)
            UploadMaxBytes = maxBytes;
        if (Get("WISBOT_UPLOAD_RETENTION_DAYS") is { } retStr && int.TryParse(retStr, out int retDays) && retDays > 0)
            UploadRetentionDays = retDays;
        if (Get("WISBOT_UPLOAD_MAX_LINKS_PER_USER") is { } linksStr && int.TryParse(linksStr, out int maxLinks) && maxLinks > 0)
            UploadMaxLinksPerUser = maxLinks;
        if (Get("WISBOT_UPLOAD_MAX_BYTES_PER_USER") is { } userBytesStr && long.TryParse(userBytesStr, out long maxUserBytes) && maxUserBytes > 0)
            UploadMaxBytesPerUser = maxUserBytes;
        if (Get("WISBOT_UPLOAD_RATE_LIMIT_PER_MINUTE") is { } rlStr && int.TryParse(rlStr, out int rl) && rl > 0)
            UploadRateLimitPerMinute = rl;

        if (Get("WISBOT_MINIO_ENDPOINT") is { } mEndpoint) MinioEndpoint = mEndpoint;
        if (Get("WISBOT_MINIO_ACCESS_KEY") is { } mAccess) MinioAccessKey = mAccess;
        if (Get("WISBOT_MINIO_SECRET_KEY") is { } mSecret) MinioSecretKey = mSecret;
        if (Get("WISBOT_MINIO_BUCKET") is { } mBucket) MinioBucket = mBucket;
        if (Get("WISBOT_MINIO_USE_SSL") is { } mSslStr && bool.TryParse(mSslStr, out bool mSsl)) MinioUseSsl = mSsl;
    }
}
