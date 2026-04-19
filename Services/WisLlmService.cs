using Discord;
using Discord.WebSocket;
using Microsoft.Data.Sqlite;
using System.Text;
using System.Text.Json;

namespace WisBot;

// ── Ollama JSON models ────────────────────────────────────────────────────────

record OllamaMessage(string Role, string Content);
record OllamaChatRequest(string Model, bool Stream, OllamaMessage[] Messages);
record OllamaChatResponse(OllamaMessage Message, bool Done);
record OllamaShowRequest(string Name);
record OllamaShowResponse(string? Parameters);
record WisLlmHistoryRow(string Username, string Prompt, string Response, bool IsCompactSummary);

// ── Service ───────────────────────────────────────────────────────────────────

/// Handles all /wisllm subcommands: ask, clear, compact.
/// Guild sessions are shared across all users in the guild.
/// DM sessions are scoped per user.
public class WisLlmService(Terminal terminal) {
    private static readonly HttpClient Http = new() { Timeout = TimeSpan.FromMinutes(5) };

    // Cache model context window sizes so we only call /api/show once per model per run
    private static readonly System.Collections.Concurrent.ConcurrentDictionary<string, int> ModelContextCache = new();

    private static readonly JsonSerializerOptions JsonOptions = new() {
        PropertyNamingPolicy        = JsonNamingPolicy.CamelCase,
        PropertyNameCaseInsensitive = true,
    };

    private const string SystemPrompt = """
        You are WisLLM, an AI assistant provided by Uhstray.io.
        Be helpful, concise, and accurate.
        This is a shared conversation — multiple users may contribute questions.
        Each user message is prefixed with [Username] so you know who asked it.
        """;

    private async Task Log(string msg, LogLevel level = LogLevel.Info) => await terminal.AddLine($"[WisLLM] {msg}", level);

    // ── Commands ─────────────────────────────────────────────────────────

    public async Task HandleAskCommand(SocketSlashCommand command) {
        var subOptions = command.Data.Options.First().Options;
        string prompt = (string)subOptions.First(o => o.Name == "prompt").Value;
        string model  = subOptions.FirstOrDefault(o => o.Name == "model")?.Value as string
                        ?? Config.OllamaDefaultModel;

        bool isEphemeral = command.Channel is IDMChannel;
        (ulong? guildId, ulong? dmUserId) = ResolveContext(command);

        await command.DeferAsync(ephemeral: isEphemeral);
        _ = Task.Run(async () => {
            try {
                long sessionId = await GetOrCreateSessionAsync(guildId, dmUserId);
                var history    = await GetContextHistoryAsync(sessionId);
                var messages   = BuildMessages(command.User.Username, prompt, history);
                string response = await CallOllamaAsync(model, messages);

                await SaveMessageAsync(sessionId, guildId, command.User.Id, command.User.Username,
                    command.Channel.Id, model, prompt, response);

                await SendChunkedAsync(command, response, isEphemeral);
                await MaybeWarnContextAsync(command, model, messages);
                await Log($"{command.User.Username} asked [{model}]: {prompt[..Math.Min(60, prompt.Length)]}");
            } catch (HttpRequestException ex) {
                await Log($"Ollama unreachable: {ex.Message}", LogLevel.Error);
                await command.FollowupAsync("Could not reach the Ollama endpoint. Is it running?", ephemeral: true);
            } catch (Exception ex) {
                await Log($"Ask error: {ex.Message}", LogLevel.Error);
                await command.FollowupAsync("Something went wrong calling WisLLM.", ephemeral: true);
            }
        });
    }

    public async Task HandleClearCommand(SocketSlashCommand command) {
        bool isEphemeral = command.Channel is IDMChannel;
        (ulong? guildId, ulong? dmUserId) = ResolveContext(command);

        await CreateNewSessionAsync(guildId, dmUserId);
        await command.RespondAsync("Conversation cleared. Fresh session started.", ephemeral: isEphemeral);
        await Log($"{command.User.Username} cleared session for {ContextLabel(guildId, dmUserId)}");
    }

    public async Task HandleCompactCommand(SocketSlashCommand command) {
        bool isEphemeral = command.Channel is IDMChannel;
        (ulong? guildId, ulong? dmUserId) = ResolveContext(command);

        await command.DeferAsync(ephemeral: isEphemeral);
        _ = Task.Run(async () => {
            try {
                long sessionId    = await GetOrCreateSessionAsync(guildId, dmUserId);
                var fullHistory   = await GetFullHistoryAsync(sessionId);

                if (fullHistory.Count == 0) {
                    await command.FollowupAsync("Nothing to compact — this session has no messages yet.", ephemeral: isEphemeral);
                    return;
                }

                string model   = Config.OllamaDefaultModel;
                string summary = await SummarizeAsync(model, fullHistory);

                long newSessionId = await CreateNewSessionAsync(guildId, dmUserId);
                await SaveMessageAsync(newSessionId, guildId, command.User.Id, command.User.Username,
                    command.Channel.Id, model,
                    prompt: "[COMPACT SUMMARY]",
                    response: summary,
                    isCompactSummary: true);

                string preview = summary.Length > 500 ? summary[..500] + "…" : summary;
                await command.FollowupAsync(
                    $"Session compacted. Continuing from the summary below.\n\n**Summary:**\n{preview}",
                    ephemeral: isEphemeral);

                await Log($"{command.User.Username} compacted session for {ContextLabel(guildId, dmUserId)}");
            } catch (HttpRequestException ex) {
                await Log($"Ollama unreachable: {ex.Message}", LogLevel.Error);
                await command.FollowupAsync("Could not reach the Ollama endpoint. Is it running?", ephemeral: true);
            } catch (Exception ex) {
                await Log($"Compact error: {ex.Message}", LogLevel.Error);
                await command.FollowupAsync("Something went wrong compacting the session.", ephemeral: true);
            }
        });
    }

    // ── Message Building ─────────────────────────────────────────────────

    private static List<OllamaMessage> BuildMessages(string username, string prompt, List<WisLlmHistoryRow> history) {
        List<OllamaMessage> messages = [new("system", SystemPrompt)];

        int start = 0;

        // Inject compact summary as additional system context rather than a fake exchange
        if (history.Count > 0 && history[0].IsCompactSummary) {
            messages.Add(new("system", $"Previous conversation summary:\n{history[0].Response}"));
            start = 1;
        }

        for (int i = start; i < history.Count; i++) {
            messages.Add(new("user",      $"[{history[i].Username}]: {history[i].Prompt}"));
            messages.Add(new("assistant", history[i].Response));
        }

        messages.Add(new("user", $"[{username}]: {prompt}"));
        return messages;
    }

    private static async Task<string> SummarizeAsync(string model, List<WisLlmHistoryRow> history) {
        List<OllamaMessage> messages = [
            new("system", "You are a helpful assistant that summarizes conversations.")
        ];

        foreach (var row in history) {
            if (row.IsCompactSummary)
                messages.Add(new("system", $"Previous summary: {row.Response}"));
            else {
                messages.Add(new("user",      $"[{row.Username}]: {row.Prompt}"));
                messages.Add(new("assistant", row.Response));
            }
        }

        messages.Add(new("user",
            "Summarize this conversation into a concise context summary that preserves all key facts, " +
            "decisions, questions asked, and conclusions reached. " +
            "This summary will be used as context for future messages."));

        return await CallOllamaAsync(model, messages);
    }

    // ── Context Window Warning ───────────────────────────────────────────

    /// Estimates token usage of the outgoing messages and warns the user if they
    /// are approaching the model's context window limit.
    private async Task MaybeWarnContextAsync(SocketSlashCommand command, string model, List<OllamaMessage> messages) {
        try {
            int contextSize   = await GetModelContextSizeAsync(model);
            int estimatedTokens = messages.Sum(m => m.Content.Length) / 4;
            int percent       = (int)((double)estimatedTokens / contextSize * 100);

            if (percent < Config.WisLlmWarnAtPercent) return;

            await command.FollowupAsync(
                $"⚠️ *This session is using ~{percent}% of **{model}**'s context window " +
                $"(~{estimatedTokens:N0} / {contextSize:N0} estimated tokens). " +
                $"Consider running `/wisllm compact` to summarise and continue.*",
                ephemeral: true);

            await Log($"Context warning sent to {command.User.Username}: {percent}% of {model} window used");
        } catch (Exception ex) {
            // Non-critical — don't let a warning failure surface to the user
            await Log($"Context warning check failed: {ex.Message}", LogLevel.Warn);
        }
    }

    /// Returns the context window size for the given model, in tokens.
    /// Priority: config file value → Ollama /api/show → hardcoded fallback (4096).
    /// Results are cached in memory for the lifetime of the process.
    private static async Task<int> GetModelContextSizeAsync(string model) {
        if (ModelContextCache.TryGetValue(model, out int cached)) return cached;

        // Config value takes priority — skips the API call entirely
        if (Config.WisLlmContextSize > 0)
            return ModelContextCache[model] = Config.WisLlmContextSize;

        const int fallback = 32768;
        try {
            var requestJson = JsonSerializer.Serialize(new OllamaShowRequest(model), JsonOptions);
            using var body  = new StringContent(requestJson, Encoding.UTF8, "application/json");

            using var cts      = new CancellationTokenSource(TimeSpan.FromSeconds(5));
            var httpResponse   = await Http.PostAsync($"{Config.OllamaEndpoint}/api/show", body, cts.Token);
            if (!httpResponse.IsSuccessStatusCode) return ModelContextCache[model] = fallback;

            var json   = await httpResponse.Content.ReadAsStringAsync();
            var parsed = JsonSerializer.Deserialize<OllamaShowResponse>(json, JsonOptions);

            // parameters is a multi-line string: "num_ctx 8192\ntemperature 0.8\n..."
            int contextSize = fallback;
            if (parsed?.Parameters != null) {
                foreach (var line in parsed.Parameters.Split('\n')) {
                    var parts = line.Trim().Split(' ', StringSplitOptions.RemoveEmptyEntries);
                    if (parts.Length == 2 && parts[0] == "num_ctx" && int.TryParse(parts[1], out int val))
                        contextSize = val;
                }
            }

            return ModelContextCache[model] = contextSize;
        } catch {
            return ModelContextCache[model] = fallback;
        }
    }

    // ── Ollama API ───────────────────────────────────────────────────────

    private static async Task<string> CallOllamaAsync(string model, List<OllamaMessage> messages) {
        var request  = new OllamaChatRequest(model, Stream: false, [.. messages]);
        var json     = JsonSerializer.Serialize(request, JsonOptions);
        using var body = new StringContent(json, Encoding.UTF8, "application/json");

        var httpResponse = await Http.PostAsync($"{Config.OllamaEndpoint}/api/chat", body);
        httpResponse.EnsureSuccessStatusCode();

        var responseJson = await httpResponse.Content.ReadAsStringAsync();
        var parsed = JsonSerializer.Deserialize<OllamaChatResponse>(responseJson, JsonOptions)
            ?? throw new InvalidOperationException("Empty response from Ollama.");

        return parsed.Message.Content;
    }

    // ── Response Chunking ────────────────────────────────────────────────

    private static async Task SendChunkedAsync(SocketSlashCommand command, string text, bool isEphemeral) {
        var chunks = ChunkText(text);
        bool multi = chunks.Count > 1;
        for (int i = 0; i < chunks.Count; i++) {
            string msg = multi ? $"({i + 1}/{chunks.Count})\n{chunks[i]}" : chunks[i];
            await command.FollowupAsync(msg, ephemeral: isEphemeral);
        }
    }

    /// Splits text into chunks no longer than maxLength.
    /// Prefers splitting at paragraph breaks, then line breaks, then spaces.
    private static List<string> ChunkText(string text, int maxLength = 1950) {
        if (text.Length <= maxLength) return [text];

        List<string> chunks = [];
        int pos = 0;

        while (pos < text.Length) {
            if (text.Length - pos <= maxLength) {
                chunks.Add(text[pos..]);
                break;
            }

            string window = text.Substring(pos, maxLength);

            int cut = window.LastIndexOf("\n\n", StringComparison.Ordinal);
            if (cut < maxLength / 2) cut = window.LastIndexOf('\n');
            if (cut < maxLength / 2) cut = window.LastIndexOf(' ');
            if (cut < maxLength / 2) cut = maxLength; // hard split as last resort

            chunks.Add(window[..cut].TrimEnd());
            pos += cut;
            while (pos < text.Length && text[pos] is '\n' or ' ') pos++;
        }

        return chunks;
    }

    // ── Session DB ───────────────────────────────────────────────────────

    private static async Task<long> GetOrCreateSessionAsync(ulong? guildId, ulong? dmUserId) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        if (guildId.HasValue) {
            cmd.CommandText = """
                SELECT id FROM wisllm_sessions
                WHERE guild_id = $guildId
                ORDER BY id DESC LIMIT 1
                """;
            cmd.Parameters.AddWithValue("$guildId", (long)guildId.Value);
        } else {
            cmd.CommandText = """
                SELECT id FROM wisllm_sessions
                WHERE guild_id IS NULL AND user_id = $userId
                ORDER BY id DESC LIMIT 1
                """;
            cmd.Parameters.AddWithValue("$userId", (long)dmUserId!.Value);
        }

        var result = await cmd.ExecuteScalarAsync();
        if (result is long existingId) return existingId;

        return await CreateNewSessionAsync(guildId, dmUserId);
    }

    private static async Task<long> CreateNewSessionAsync(ulong? guildId, ulong? dmUserId) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            INSERT INTO wisllm_sessions (guild_id, user_id, created_at)
            VALUES ($guildId, $userId, $now)
            RETURNING id
            """;
        cmd.Parameters.AddWithValue("$guildId", guildId.HasValue  ? (object)(long)guildId.Value  : DBNull.Value);
        cmd.Parameters.AddWithValue("$userId",  dmUserId.HasValue ? (object)(long)dmUserId.Value : DBNull.Value);
        cmd.Parameters.AddWithValue("$now", DateTime.UtcNow.ToString("O"));

        return (long)(await cmd.ExecuteScalarAsync())!;
    }

    // ── History DB ───────────────────────────────────────────────────────

    /// Returns the last N messages in the session in chronological order.
    private static async Task<List<WisLlmHistoryRow>> GetContextHistoryAsync(long sessionId) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            SELECT username, prompt, response, is_compact_summary
            FROM wisllm_history
            WHERE session_id = $sessionId
            ORDER BY timestamp DESC
            LIMIT $limit
            """;
        cmd.Parameters.AddWithValue("$sessionId", sessionId);
        cmd.Parameters.AddWithValue("$limit", Config.WisLlmContextLimit);

        List<WisLlmHistoryRow> rows = [];
        using var reader = await cmd.ExecuteReaderAsync();
        while (await reader.ReadAsync())
            rows.Add(new WisLlmHistoryRow(
                Username:          reader.GetString(0),
                Prompt:            reader.GetString(1),
                Response:          reader.GetString(2),
                IsCompactSummary:  reader.GetInt64(3) == 1
            ));

        rows.Reverse(); // DESC fetch → reverse for chronological order
        return rows;
    }

    /// Returns all messages in the session in chronological order (used for compact).
    private static async Task<List<WisLlmHistoryRow>> GetFullHistoryAsync(long sessionId) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            SELECT username, prompt, response, is_compact_summary
            FROM wisllm_history
            WHERE session_id = $sessionId
            ORDER BY timestamp ASC
            """;
        cmd.Parameters.AddWithValue("$sessionId", sessionId);

        List<WisLlmHistoryRow> rows = [];
        using var reader = await cmd.ExecuteReaderAsync();
        while (await reader.ReadAsync())
            rows.Add(new WisLlmHistoryRow(
                Username:         reader.GetString(0),
                Prompt:           reader.GetString(1),
                Response:         reader.GetString(2),
                IsCompactSummary: reader.GetInt64(3) == 1
            ));

        return rows;
    }

    private static async Task SaveMessageAsync(long sessionId, ulong? guildId, ulong userId,
        string username, ulong channelId, string model, string prompt, string response,
        bool isCompactSummary = false) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            INSERT INTO wisllm_history
                (session_id, guild_id, user_id, username, channel_id, model, prompt, response, timestamp, is_compact_summary)
            VALUES
                ($sessionId, $guildId, $userId, $username, $channelId, $model, $prompt, $response, $now, $isCompactSummary)
            """;
        cmd.Parameters.AddWithValue("$sessionId",        sessionId);
        cmd.Parameters.AddWithValue("$guildId",          guildId.HasValue ? (object)(long)guildId.Value : DBNull.Value);
        cmd.Parameters.AddWithValue("$userId",           (long)userId);
        cmd.Parameters.AddWithValue("$username",         username);
        cmd.Parameters.AddWithValue("$channelId",        (long)channelId);
        cmd.Parameters.AddWithValue("$model",            model);
        cmd.Parameters.AddWithValue("$prompt",           prompt);
        cmd.Parameters.AddWithValue("$response",         response);
        cmd.Parameters.AddWithValue("$now",              DateTime.UtcNow.ToString("O"));
        cmd.Parameters.AddWithValue("$isCompactSummary", isCompactSummary ? 1 : 0);

        await cmd.ExecuteNonQueryAsync();
    }

    // ── Helpers ──────────────────────────────────────────────────────────

    private static (ulong? guildId, ulong? dmUserId) ResolveContext(SocketSlashCommand command) =>
        command.Channel is IDMChannel
            ? (null, command.User.Id)
            : ((command.Channel as SocketGuildChannel)!.Guild.Id, (ulong?)null);

    private static string ContextLabel(ulong? guildId, ulong? dmUserId) =>
        guildId.HasValue ? $"guild {guildId}" : $"DM user {dmUserId}";
}
