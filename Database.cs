using Microsoft.Data.Sqlite;

/// Central database helper. All features share the same wisbot.db file.
/// Call Database.Initialize() once on startup to ensure all tables exist.
public static class Database {
    private const string DbPath = "wisbot.db";
    public static string ConnectionString { get; } = $"Data Source={DbPath}";

    public static async Task Initialize() {
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
            """;
        await cmd.ExecuteNonQueryAsync();
    }
}
