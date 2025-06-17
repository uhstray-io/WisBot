package main

import (
	"context"
	"fmt"
	"time"
	"wisbot/src/sqlc"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
)

// Global database query handler
var db *sqlc.Queries

// GetDBQueries returns the initialized database queries object.
func GetDBQueries() (*sqlc.Queries, error) {
	if db == nil {
		return nil, fmt.Errorf("database queries are not initialized")
	}
	return db, nil
}

// StartDatabaseService initializes the database connection and setup
func StartDatabaseService(ctx context.Context) {
	ctx, span := StartSpan(ctx, "database.StartDatabase")
	defer span.End()

	LogInfo(ctx, "Connecting to database")
	conn, err := pgx.Connect(ctx, databaseUrl)
	if err != nil {
		span.RecordError(err)
		PanicError(ctx, err, "Error while connecting to database")
	}

	db = sqlc.New(conn)
	// Create the tables if they don't exist
	if err := db.CreateFilesTable(ctx); err != nil {
		span.RecordError(err)
		PanicError(ctx, err, "Error creating files table")
	}

	LogInfo(ctx, "Database successfully initialized")

	StartDatabaseCleanup(ctx, conn)
}

// StartDatabaseCleanup begins a periodic task that removes old files
func StartDatabaseCleanup(ctx context.Context, db *pgx.Conn) {
	ctx, span := StartSpan(ctx, "database.StartDatabaseCleanup")
	defer span.End()

	LogInfo(ctx, "Starting database cleanup process", attribute.Int("delete_older_than_days", deleteFilesAfterDays))

	go func() {
		time.Sleep(5 * time.Second)

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		cleanupCtx, cleanupSpan := StartSpan(ctx, "database.initialCleanup")
		if err := runCleanup(cleanupCtx); err != nil {
			LogError(ctx, err, "Error in initial cleanup")
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
				LogInfo(ctx, "Database cleanup process stopping due to context cancellation.")
				return // Exit the goroutine
			}
		}
	}()
}

// runCleanup performs a single database cleanup operation
func runCleanup(ctx context.Context) error {
	if db == nil {
		return nil
	}

	ctx, span := StartSpan(ctx, "database.runCleanup")
	defer span.End()

	LogInfo(ctx, "Running database cleanup process", attribute.Int("delete_older_than_days", deleteFilesAfterDays))

	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := db.DeleteFileWhereOlderThan(cleanupCtx, int32(deleteFilesAfterDays)); err != nil {
		span.RecordError(err)
		err = fmt.Errorf("error while executing DeleteFileWhereOlderThan query: %w", err)
		LogError(ctx, err, "Cleanup failed")
		return err
	}

	return nil
}
