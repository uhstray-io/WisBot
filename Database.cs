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
                UNIQUE (watcher_id, target_id, guild_id)
            );
            """;
        await cmd.ExecuteNonQueryAsync();
    }
}
