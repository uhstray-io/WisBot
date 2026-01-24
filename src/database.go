package main

import (
	"context"
	"fmt"
	"time"
	"wisbot/src/sqlc"

	"github.com/jackc/pgx/v5"
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
	LogInfo("Connecting to database")

	conn, err := pgx.Connect(ctx, databaseUrl)
	if err != nil {
		PanicError(err, "Error while connecting to database")
	}

	db = sqlc.New(conn)
	// Create the tables if they don't exist
	if err := db.CreateFilesTable(ctx); err != nil {

		PanicError(err, "Error creating files table")
	}

	LogInfo("Database successfully initialized")

	StartDatabaseCleanup(ctx, conn)
}

// StartDatabaseCleanup begins a periodic task that removes old files
func StartDatabaseCleanup(ctx context.Context, db *pgx.Conn) {

	LogInfo("Starting database cleanup process")

	go func() {
		time.Sleep(5 * time.Second)

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		if err := runCleanup(ctx); err != nil {
			LogError(err, "Error in initial cleanup")
		}

		for {
			select {
			case <-ticker.C:
				if err := runCleanup(ctx); err != nil {
					LogError(err, "Error in scheduled database cleanup")
				}

			case <-ctx.Done():
				LogInfo("Database cleanup process stopping due to context cancellation.")
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

	LogInfo("Running database cleanup process")

	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := db.DeleteFileWhereOlderThan(cleanupCtx, int32(deleteFilesAfterDays)); err != nil {
		LogError(err, "Cleanup failed while executing DeleteFileWhereOlderThan query")
		return err
	}

	return nil
}
