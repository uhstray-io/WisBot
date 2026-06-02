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

    // ── Retention loop ───────────────────────────────────────────────────

    private CancellationTokenSource? retentionCts;

    /// Starts the hourly retention sweep (deletes expired uploads + their objects).
    public void StartRetention() {
        if (!Config.UploadEnabled) return;
        retentionCts = new CancellationTokenSource();
        _ = Task.Run(() => RunRetentionLoop(retentionCts.Token));
    }

    public void StopRetention() {
        retentionCts?.Cancel();
        retentionCts?.Dispose();
        retentionCts = null;
    }

    private async Task RunRetentionLoop(CancellationToken token) {
        while (!token.IsCancellationRequested) {
            try {
                int removed = await CleanupExpiredAsync();
                if (removed > 0) await Log($"Retention: removed {removed} expired upload(s)");
                await Task.Delay(TimeSpan.FromHours(1), token);
            } catch (OperationCanceledException) {
                break;
            } catch (Exception ex) {
                await Log($"Retention loop error: {ex.Message}", LogLevel.Error);
                await Task.Delay(TimeSpan.FromHours(1), token);
            }
        }
    }

    /// Removes the MinIO object (if any) and DB row for every expired upload.
    private async Task<int> CleanupExpiredAsync() {
        List<(string Id, string Status)> expired = [];

        using (var conn = new SqliteConnection(Database.ConnectionString)) {
            await conn.OpenAsync();
            var sel = conn.CreateCommand();
            sel.CommandText = "SELECT id, status FROM uploads WHERE expires_at IS NOT NULL AND expires_at <= $now";
            sel.Parameters.AddWithValue("$now", DateTime.UtcNow.ToString("O"));
            using var reader = await sel.ExecuteReaderAsync();
            while (await reader.ReadAsync())
                expired.Add((reader.GetString(0), reader.GetString(1)));
        }

        foreach (var (id, status) in expired) {
            if (status is "ready" or "uploading")
                await TryRemoveObjectAsync(id);
            await DeleteRowAsync(id);
        }
        return expired.Count;
    }

    private async Task TryRemoveObjectAsync(string id) {
        try {
            await Minio().RemoveObjectAsync(new RemoveObjectArgs()
                .WithBucket(Config.MinioBucket).WithObject(id));
        } catch (Exception ex) {
            // Don't log the id (bearer credential); best-effort cleanup.
            await Log($"Retention: could not remove an expired object: {ex.Message}", LogLevel.Warn);
        }
    }

    private static async Task DeleteRowAsync(string id) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();
        var cmd = conn.CreateCommand();
        cmd.CommandText = "DELETE FROM uploads WHERE id = $id";
        cmd.Parameters.AddWithValue("$id", id);
        await cmd.ExecuteNonQueryAsync();
    }

    // ── Command ──────────────────────────────────────────────────────────

    public async Task HandleUploadCommand(SocketSlashCommand command) {
        if (!Config.UploadEnabled) {
            await command.RespondAsync("File uploads aren't configured on this bot right now.", ephemeral: true);
            return;
        }

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
    /// Atomically claims the link (pending → uploading) first so a single-use link
    /// can't be filled twice by concurrent POSTs. Returns false if it wasn't claimable
    /// (already used or in progress).
    public async Task<bool> StoreAsync(string id, Stream content, string filename, string contentType, long size) {
        if (!await ClaimForUploadAsync(id)) return false;

        try {
            await EnsureBucketAsync();
            string type = string.IsNullOrWhiteSpace(contentType) ? "application/octet-stream" : contentType;

            await Minio().PutObjectAsync(new PutObjectArgs()
                .WithBucket(Config.MinioBucket)
                .WithObject(id)
                .WithStreamData(content)
                .WithObjectSize(size)
                .WithContentType(type));

            await MarkReadyAsync(id, filename, type, size);
            await Log($"Stored an upload ({size / 1024} KB)");
            return true;
        } catch {
            await RevertToPendingAsync(id); // let the user retry the link
            throw;
        }
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

    /// Atomic claim: flips pending → uploading. Returns true only for the caller
    /// that won the race (rows affected > 0).
    private static async Task<bool> ClaimForUploadAsync(string id) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = "UPDATE uploads SET status = 'uploading' WHERE id = $id AND status = 'pending'";
        cmd.Parameters.AddWithValue("$id", id);
        return await cmd.ExecuteNonQueryAsync() > 0;
    }

    private static async Task RevertToPendingAsync(string id) {
        using var conn = new SqliteConnection(Database.ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = "UPDATE uploads SET status = 'pending' WHERE id = $id AND status = 'uploading'";
        cmd.Parameters.AddWithValue("$id", id);
        await cmd.ExecuteNonQueryAsync();
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
            WHERE id = $id AND status = 'uploading'
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
