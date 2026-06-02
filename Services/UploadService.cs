using Discord.WebSocket;
using Microsoft.Data.Sqlite;
using Minio;
using Minio.DataModel.Args;
using System.Security.Cryptography;

namespace WisBot;

/// One upload row's metadata (status + file info once ready).
public record UploadRecord(string Id, string Status, string? Filename, string? ContentType, long? SizeBytes);

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

    // ── Storage (MinIO) ──────────────────────────────────────────────────

    private IMinioClient? minio;

    private IMinioClient Minio() => minio ??= new MinioClient()
        .WithEndpoint(Config.MinioEndpoint)
        .WithCredentials(Config.MinioAccessKey, Config.MinioSecretKey)
        .WithSSL(Config.MinioUseSsl)
        .Build();

    private async Task EnsureBucketAsync() {
        var client = Minio();
        bool exists = await client.BucketExistsAsync(
            new BucketExistsArgs().WithBucket(Config.MinioBucket));
        if (!exists)
            await client.MakeBucketAsync(new MakeBucketArgs().WithBucket(Config.MinioBucket));
    }

    /// Streams the uploaded file into MinIO (object key = id) and marks the row ready.
    public async Task StoreAsync(string id, Stream content, string filename, string contentType, long size) {
        await EnsureBucketAsync();
        string type = string.IsNullOrWhiteSpace(contentType) ? "application/octet-stream" : contentType;

        await Minio().PutObjectAsync(new PutObjectArgs()
            .WithBucket(Config.MinioBucket)
            .WithObject(id)
            .WithStreamData(content)
            .WithObjectSize(size)
            .WithContentType(type));

        await MarkReadyAsync(id, filename, type, size);
        await Log($"Stored upload {id[..Math.Min(6, id.Length)]}… ({size / 1024} KB)");
    }

    /// Streams the stored object to the response output stream.
    public async Task DownloadToAsync(string id, Stream output, CancellationToken cancellationToken = default) {
        await Minio().GetObjectAsync(new GetObjectArgs()
            .WithBucket(Config.MinioBucket)
            .WithObject(id)
            .WithCallbackStream(async (stream, token) => await stream.CopyToAsync(output, token)),
            cancellationToken);
    }

    // ── DB ───────────────────────────────────────────────────────────────

    public async Task<UploadRecord?> GetUploadAsync(string id) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            SELECT status, filename, content_type, size_bytes
            FROM uploads WHERE id = $id
            """;
        cmd.Parameters.AddWithValue("$id", id);

        using var reader = await cmd.ExecuteReaderAsync();
        if (!await reader.ReadAsync()) return null;

        return new UploadRecord(
            Id: id,
            Status: reader.GetString(0),
            Filename: reader.IsDBNull(1) ? null : reader.GetString(1),
            ContentType: reader.IsDBNull(2) ? null : reader.GetString(2),
            SizeBytes: reader.IsDBNull(3) ? null : reader.GetInt64(3));
    }

    private static async Task MarkReadyAsync(string id, string filename, string contentType, long size) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        DateTime now = DateTime.UtcNow;
        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            UPDATE uploads
            SET status = 'ready', filename = $f, content_type = $ct, size_bytes = $sz,
                uploaded_at = $now, expires_at = $exp
            WHERE id = $id
            """;
        cmd.Parameters.AddWithValue("$f", filename);
        cmd.Parameters.AddWithValue("$ct", contentType);
        cmd.Parameters.AddWithValue("$sz", size);
        cmd.Parameters.AddWithValue("$now", now.ToString("O"));
        // Retention counts from upload, as advertised to the user.
        cmd.Parameters.AddWithValue("$exp", now.AddDays(Config.UploadRetentionDays).ToString("O"));
        cmd.Parameters.AddWithValue("$id", id);

        await cmd.ExecuteNonQueryAsync();
    }

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
