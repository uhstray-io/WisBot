package main

import (
	"context"
	"fmt"
	"time"
	"wisbot/src/sqlgo"

	"github.com/jackc/pgx/v5"
)

// Global database query handler
var wisQueries *sqlgo.Queries

// StartDatabase initializes the database connection and creates required tables
func StartDatabase() (*pgx.Conn, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseUrl)
	if err != nil {
		return nil, fmt.Errorf("error while connecting to database: %w", err)
	}

	wisQueries = sqlgo.New(conn)

	// Create the tables if they don't exist
	err = wisQueries.CreateFilesTable(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating files table: %w", err)
	}

	return conn, nil
}

// StartDatabaseCleanup begins a periodic task that removes old files
func StartDatabaseCleanup(db *pgx.Conn) {
	go func() {
		// Initial delay before starting cleanup
		time.Sleep(5 * time.Second)

		// Set up ticker for periodic cleanup
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		// Run cleanup immediately, then on each tick
		if err := runCleanup(); err != nil {
			fmt.Printf("Error in initial cleanup: %v\n", err)
		}

		for range ticker.C {
			if err := runCleanup(); err != nil {
				fmt.Printf("error in scheduled cleanup: %v\n", err)
			}
		}
	}()
}

// runCleanup performs a single database cleanup operation
func runCleanup() error {
	fmt.Println("Running database cleanup process")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := wisQueries.DeleteFileWhereOlderThan(ctx, int32(deleteFilesAfterDays))
	if err != nil {
		return fmt.Errorf("error while executing DeleteFileWhereOlderThan query: %w", err)
	}

	return nil
}
