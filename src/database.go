package main

import (
	"context"
	"fmt"
	"time"
	"wisbot/src/sqlgo"

	"github.com/jackc/pgx/v5"
	"github.com/rotisserie/eris"
)

// Global database query handler
var wisQueries *sqlgo.Queries

// StartDatabase initializes the database connection and creates required tables
func StartDatabase() (*pgx.Conn, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseUrl)
	if err != nil {
		return nil, eris.Wrapf(err, "Error connecting to database: %v", err)
	}

	wisQueries = sqlgo.New(conn)

	// Create the tables if they don't exist
	err = wisQueries.CreateFilesTable(ctx)
	if err != nil {
		return nil, eris.Wrap(err, "Error creating files table")
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
				fmt.Printf("Error in scheduled cleanup: %v\n", err)
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
		return eris.Wrap(err, "Error executing DeleteFileWhereOlderThan query")
	}

	return nil
}
