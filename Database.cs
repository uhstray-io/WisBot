using Microsoft.Data.Sqlite;

namespace WisBot;

/// Central database helper. All features share the same wisbot.db file.
/// Call Database.Initialize() once on startup to ensure all tables exist.
///
/// INVARIANT: every DateTime column (remind_at, expires_at, timestamp, created_at, …)
/// stores DateTime.UtcNow.ToString("O") — fixed-width, Z-suffixed, lexicographically
/// sortable. Due-time queries compare these as TEXT (e.g. remind_at <= $now), which is
/// only correct while ALL writers use UTC "O" strings. Never store a Local/Unspecified
/// DateTime; its "O" form has a different shape and breaks the ordering.
public static class Database {
    // Resolved from Config (env-configurable) so the DB can live on a mounted volume.
    public static string ConnectionString => $"Data Source={Config.DbPath}";

    public static async Task Initialize() {
        // An operator-supplied WISBOT_DB_PATH may point into a not-yet-created directory
        // (e.g. an unmounted volume) — fail with a clear mkdir rather than an opaque
        // 'unable to open database file'.
        var dbDir = Path.GetDirectoryName(Path.GetFullPath(Config.DbPath));
        if (!string.IsNullOrEmpty(dbDir)) Directory.CreateDirectory(dbDir);

        using var conn = new SqliteConnection(ConnectionString);
        await conn.OpenAsync();

        var cmd = conn.CreateCommand();
        cmd.CommandText = """
            CREATE TABLE IF NOT EXISTS welcomed_users (
                guild_id INTEGER NOT NULL,
                user_id  INTEGER NOT NULL,
                PRIMARY KEY (guild_id, user_id)
            );

            CREATE TABLE IF NOT EXISTS reminders (
                id         INTEGER PRIMARY KEY AUTOINCREMENT,
                user_id    INTEGER NOT NULL,
                channel_id INTEGER NOT NULL,
                message    TEXT    NOT NULL,
                remind_at  TEXT    NOT NULL
            );

            CREATE TABLE IF NOT EXISTS voice_notifications (
                id         INTEGER PRIMARY KEY AUTOINCREMENT,
                watcher_id INTEGER NOT NULL,
                target_id  INTEGER NOT NULL,
                guild_id   INTEGER NOT NULL,
                is_active  INTEGER NOT NULL DEFAULT 1,
                UNIQUE (watcher_id, target_id, guild_id)
            );

            CREATE TABLE IF NOT EXISTS voice_activity (
                id           INTEGER PRIMARY KEY AUTOINCREMENT,
                user_id      INTEGER NOT NULL,
                username     TEXT    NOT NULL,
                guild_id     INTEGER NOT NULL,
                guild_name   TEXT    NOT NULL,
                channel_id   INTEGER NOT NULL,
                channel_name TEXT    NOT NULL,
                action       TEXT    NOT NULL,
                timestamp    TEXT    NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_voice_activity_user_guild
                ON voice_activity (user_id, guild_id);

            CREATE TABLE IF NOT EXISTS wisllm_sessions (
                id         INTEGER PRIMARY KEY AUTOINCREMENT,
                guild_id   INTEGER,
                user_id    INTEGER,
                created_at TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_wisllm_sessions_guild ON wisllm_sessions (guild_id);
            CREATE INDEX IF NOT EXISTS idx_wisllm_sessions_user  ON wisllm_sessions (user_id);

            CREATE TABLE IF NOT EXISTS wisllm_history (
                id                 INTEGER PRIMARY KEY AUTOINCREMENT,
                session_id         INTEGER NOT NULL REFERENCES wisllm_sessions(id),
                guild_id           INTEGER,
                user_id            INTEGER NOT NULL,
                username           TEXT    NOT NULL,
                channel_id         INTEGER NOT NULL,
                model              TEXT    NOT NULL,
                prompt             TEXT    NOT NULL,
                response           TEXT    NOT NULL,
                timestamp          TEXT    NOT NULL,
                is_compact_summary INTEGER NOT NULL DEFAULT 0
            );

            CREATE INDEX IF NOT EXISTS idx_wisllm_history_session
                ON wisllm_history (session_id);

            CREATE TABLE IF NOT EXISTS uploads (
                id             TEXT    PRIMARY KEY,
                owner_user_id  INTEGER NOT NULL,
                owner_username TEXT    NOT NULL,
                filename       TEXT,
                content_type   TEXT,
                size_bytes     INTEGER,
                status         TEXT    NOT NULL DEFAULT 'pending',
                created_at     TEXT    NOT NULL,
                uploaded_at    TEXT,
                expires_at     TEXT
            );
            CREATE INDEX IF NOT EXISTS idx_uploads_expires ON uploads (expires_at);
            """;
        await cmd.ExecuteNonQueryAsync();
    }
}
