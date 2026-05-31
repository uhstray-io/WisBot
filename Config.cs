namespace WisBot;

/// Loads configuration from a .env file and discord.key at startup.
/// Falls back to safe defaults if the file or a key is missing.
public static class Config {
    public static string DiscordToken { get; private set; } = string.Empty;
    public static string OllamaEndpoint { get; private set; } = "http://localhost:11434";
    public static string OllamaDefaultModel { get; private set; } = "llama3";
    public static int WisLlmContextLimit { get; private set; } = 10;
    public static int WisLlmWarnAtPercent { get; private set; } = 75;
    public static int WisLlmContextSize { get; private set; } = 0;

    public static void Load(string envPath = ".env", string tokenPath = "discord.key") {
        // discord.key is the local dev path; .env is the Docker/CI path
        if (File.Exists(tokenPath))
            DiscordToken = File.ReadAllText(tokenPath).Trim();

        if (!File.Exists(envPath)) return;

        foreach (var raw in File.ReadAllLines(envPath)) {
            var line = raw.Trim();
            if (line.StartsWith('#') || !line.Contains('=')) continue;

            int idx = line.IndexOf('=');
            var key = line[..idx].Trim();
            var value = line[(idx + 1)..].Trim();

            switch (key) {
                // Deployment workflow (deployment_prod.yml) appends DISCORD_TOKEN_WISBOT to .env
                case "DISCORD_TOKEN_WISBOT":
                    if (!string.IsNullOrWhiteSpace(value)) DiscordToken = value;
                    break;
                case "OLLAMA_ENDPOINT":
                    if (!string.IsNullOrWhiteSpace(value)) OllamaEndpoint = value;
                    break;
                case "OLLAMA_DEFAULT_MODEL":
                    if (!string.IsNullOrWhiteSpace(value)) OllamaDefaultModel = value;
                    break;
                case "WISLLM_CONTEXT_LIMIT":
                    if (int.TryParse(value, out int limit) && limit > 0)
                        WisLlmContextLimit = limit;
                    break;
                case "WISLLM_WARN_AT_PERCENT":
                    if (int.TryParse(value, out int pct) && pct is > 0 and <= 100)
                        WisLlmWarnAtPercent = pct;
                    break;
                case "WISLLM_CONTEXT_SIZE":
                    if (int.TryParse(value, out int size) && size > 0)
                        WisLlmContextSize = size;
                    break;
            }
        }
    }
}
