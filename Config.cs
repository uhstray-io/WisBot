/// Loads configuration from a .env file at startup.
/// Falls back to safe defaults if the file or a key is missing.
public static class Config {
    public static string OllamaEndpoint        { get; private set; } = "http://localhost:11434";
    public static string OllamaDefaultModel    { get; private set; } = "llama3";
    public static int    WisLlmContextLimit    { get; private set; } = 10;
    /// Warn the user when estimated token usage exceeds this % of the model's context window.
    public static int    WisLlmWarnAtPercent   { get; private set; } = 75;
    /// Hard-coded context window size in tokens. When set, skips the Ollama /api/show query.
    /// Leave unset (0) to query Ollama dynamically.
    public static int    WisLlmContextSize     { get; private set; } = 0;

    public static void Load(string path = ".env") {
        if (!File.Exists(path)) return;

        foreach (var raw in File.ReadAllLines(path)) {
            var line = raw.Trim();
            if (line.StartsWith('#') || !line.Contains('=')) continue;

            int idx  = line.IndexOf('=');
            var key  = line[..idx].Trim();
            var value = line[(idx + 1)..].Trim();

            switch (key) {
                case "OLLAMA_ENDPOINT":       OllamaEndpoint     = value; break;
                case "OLLAMA_DEFAULT_MODEL":  OllamaDefaultModel = value; break;
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
