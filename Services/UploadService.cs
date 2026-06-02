using Discord.WebSocket;
using Microsoft.Data.Sqlite;
using System.Security.Cryptography;

namespace WisBot;

/// Handles the /upload slash command — mints an unguessable upload link backed by
/// a `pending` row in the `uploads` table. The web layer (WebService) turns the
/// link into an upload page, then a download page once a file is stored.
/// Phase 8 file relay: bypasses Discord's attachment size limit.
public class UploadService(Terminal terminal) {
    private async Task Log(string msg, LogLevel level = LogLevel.Info)
        => await terminal.AddLine($"[Upload] {msg}", level);

    // ── Command ──────────────────────────────────────────────────────────

    public async Task HandleUploadCommand(SocketSlashCommand command) {
        string id = GenerateId();
        await CreateUpload(id, command.User.Id, command.User.Username);

        long maxMb = Config.UploadMaxBytes / (1024 * 1024);
        var link = $"{Config.PublicBaseUrl}/u/{id}";
        await command.RespondAsync(
            $"📤 Your upload link (one file, up to {maxMb} MB, kept {Config.UploadRetentionDays} days):\n{link}\n" +
            "Open it to upload your file — then share the same link so anyone can download it.",
            ephemeral: true);

        // Do not log the id — it is the bearer credential for the link.
        await Log($"{command.User.Username} minted an upload link");
    }

    // ── Helpers ──────────────────────────────────────────────────────────

    /// 128 bits of CSPRNG entropy as a URL-safe (base64url, no padding) token.
    private static string GenerateId() {
        Span<byte> bytes = stackalloc byte[16];
        RandomNumberGenerator.Fill(bytes);
        return Convert.ToBase64String(bytes).Replace('+', '-').Replace('/', '_').TrimEnd('=');
    }

    // ── DB ───────────────────────────────────────────────────────────────

    private static async Task CreateUpload(string id, ulong ownerId, string ownerName) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        DateTime now = DateTime.UtcNow;

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            INSERT INTO uploads (id, owner_user_id, owner_username, status, created_at, expires_at)
            VALUES ($id, $owner, $name, 'pending', $now, $expires)
            """;
        cmd.Parameters.AddWithValue("$id", id);
        cmd.Parameters.AddWithValue("$owner", (long)ownerId);
        cmd.Parameters.AddWithValue("$name", ownerName);
        cmd.Parameters.AddWithValue("$now", now.ToString("O"));
        // Set expiry at mint so unused links age out; refreshed on upload (8a-3).
        cmd.Parameters.AddWithValue("$expires", now.AddDays(Config.UploadRetentionDays).ToString("O"));

        await cmd.ExecuteNonQueryAsync();
    }
}
