package main

import (
	"context"
	"fmt"
	"time"
	"wisbot/src/sqlgo"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
)

// Global database query handler
var wisQueries *sqlgo.Queries

// StartDatabase initializes the database connection and creates required tables
func StartDatabase(ctx context.Context) (*pgx.Conn, error) {
	ctx, span := StartSpan(ctx, "database.StartDatabase")
	defer span.End()

	LogEvent(ctx, log.SeverityInfo, "Connecting to database")

	conn, err := pgx.Connect(ctx, databaseUrl)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("error while connecting to database: %w", err)
	}

	wisQueries = sqlgo.New(conn)

	// Create the tables if they don't exist
	err = wisQueries.CreateFilesTable(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("error creating files table: %w", err)
	}

	LogEvent(ctx, log.SeverityInfo, "Database successfully initialized")

	return conn, nil
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
		cleanupCtx, _ := StartSpan(ctx, "database.initialCleanup")
		if err := runCleanup(cleanupCtx); err != nil {
			fmt.Printf("Error in initial cleanup: %v\n", err)
		}

		for {
			select {
			case <-ticker.C:
				tickCtx, _ := StartSpan(ctx, "database.scheduledCleanup")
				if err := runCleanup(tickCtx); err != nil {
					fmt.Printf("error in scheduled cleanup: %v\n", err)
				}
			case <-ctx.Done():
				LogEvent(ctx, log.SeverityInfo, "Stopping database cleanup process")
				return
			}
		}
	}()
}

// runCleanup performs a single database cleanup operation
func runCleanup(ctx context.Context) error {
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
