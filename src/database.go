package main

import (
	"context"
	"fmt"
	"time"
	"wisbot/src/sqlc"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
)

// Global database query handler
var wisQueries *sqlc.Queries

// GetDBQueries returns the initialized database queries object.
// It returns an error if the database service is not enabled or not initialized.
func GetDBQueries() (*sqlc.Queries, error) {
	if !postgresServiceEnabled {
		return nil, fmt.Errorf("database service is not enabled")
	}
	if wisQueries == nil {
		return nil, fmt.Errorf("database queries are not initialized")
	}
	return wisQueries, nil
}

// StartDatabaseService initializes the database connection and setup
func StartDatabaseService(ctx context.Context) {
	if !postgresServiceEnabled {
		fmt.Println("Postgres service is disabled. Skipping database initialization.")
		return
	}

	ctx, span := StartSpan(ctx, "database.StartDatabase")
	defer span.End()

	LogEvent(ctx, log.SeverityInfo, "Connecting to database")

	conn, err := pgx.Connect(ctx, databaseUrl)
	if err != nil {
		span.RecordError(err)
		panic(fmt.Errorf("error while connecting to database: %w", err))
	}

	wisQueries = sqlc.New(conn)

	// Create the tables if they don't exist
	err = wisQueries.CreateFilesTable(ctx)
	if err != nil {
		span.RecordError(err)
		panic(fmt.Errorf("error creating files table: %w", err))
	}

	LogEvent(ctx, log.SeverityInfo, "Database successfully initialized")

	// defer conn.Close(context.Background())

	go StartDatabaseCleanup(ctx, conn)

}

// StartDatabaseCleanup begins a periodic task that removes old files
func StartDatabaseCleanup(ctx context.Context, db *pgx.Conn) {
	ctx, span := StartSpan(ctx, "database.StartDatabaseCleanup")
	defer span.End()

	LogEvent(ctx, log.SeverityInfo, "Starting database cleanup process", attribute.Int("delete_older_than_days", deleteFilesAfterDays))

	go func() {
		// Initial delay before starting cleanup
		time.Sleep(5 * time.Second)

		// Set up ticker for periodic cleanup
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		// Run cleanup immediately, then on each tick
		cleanupCtx, cleanupSpan := StartSpan(ctx, "database.initialCleanup")
		if err := runCleanup(cleanupCtx); err != nil {
			fmt.Printf("Error in initial cleanup: %v\n", err)
		}
		cleanupSpan.End()

		for {
			select {
			case <-ticker.C:
				tickCtx, tickSpan := StartSpan(ctx, "database.scheduledCleanup")
				if err := runCleanup(tickCtx); err != nil {
					LogError(tickCtx, err, "Error in scheduled database cleanup")
				}
				tickSpan.End()
			case <-ctx.Done():
				LogEvent(ctx, log.SeverityInfo, "Database cleanup process stopping due to context cancellation.")
				return // Exit the goroutine
			}
		}
	}()
}

// runCleanup performs a single database cleanup operation
func runCleanup(ctx context.Context) error {
	if wisQueries == nil {
		return nil
	}

	ctx, span := StartSpan(ctx, "database.runCleanup")
	defer span.End()

	LogEvent(ctx, log.SeverityInfo, "Running database cleanup process", attribute.Int("delete_older_than_days", deleteFilesAfterDays))

	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := wisQueries.DeleteFileWhereOlderThan(cleanupCtx, int32(deleteFilesAfterDays))
	if err != nil {
		span.RecordError(err)
		LogError(ctx, err, "Cleanup failed")
		return fmt.Errorf("error while executing DeleteFileWhereOlderThan query: %w", err)
	}

	return nil
}
