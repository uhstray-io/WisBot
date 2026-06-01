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

    // HTTP health endpoint. Host defaults to "localhost" (dev); set "+" in the
    // container so Docker port mapping can reach it.
    public static string HealthHost { get; private set; } = "localhost";
    public static int HealthPort { get; private set; } = 8080;

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

        if (Get("WISBOT_HEALTH_HOST") is { } healthHost) HealthHost = healthHost;
        if (Get("WISBOT_HEALTH_PORT") is { } healthPortStr && int.TryParse(healthPortStr, out int healthPort) && healthPort is > 0 and <= 65535)
            HealthPort = healthPort;
    }
}
