package main

import (
	"context"
	"fmt"
	"os"
	"time"
	"wisbot/src/sqlgo"

	"github.com/jackc/pgx/v5"
)

// var databaseConnection *pgx.Conn
var wisQueries *sqlgo.Queries

func StartDatabase() *pgx.Conn {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	wisQueries = sqlgo.New(conn)

	// Create the tables if it does not exist.
	wisQueries.CreateFilesTable(ctx)

	return conn
}

// Delete old files ticker
func DatabaseCleanup(db *pgx.Conn) {

	DeleteOldFilesHandler := func(db *pgx.Conn) {
		err := wisQueries.DeleteFileWhereOlderThan(context.Background(), int32(deleteFilesAfterDays))

		if err != nil {
			fmt.Println("Error while executing DeleteFileWhereOlderThan query", err)
		}
	}

	time.Sleep(5 * time.Second)

	go func() {
		// Trigger cleanup process every hour.
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			fmt.Println("Running cleanup process")
			DeleteOldFilesHandler(db)
			<-ticker.C
		}
	}()
}
