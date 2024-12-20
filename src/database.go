package main

import (
	"context"
	"fmt"
	"time"
	"wisbot/src/sqlgo"

	"github.com/jackc/pgx/v5"
	"github.com/rotisserie/eris"
)

// var databaseConnection *pgx.Conn
var wisQueries *sqlgo.Queries

func StartDatabase() (*pgx.Conn, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseUrl)
	if err != nil {
		return nil, eris.Wrapf(err, "Error while connecting to database: %v", err)
	}

	wisQueries = sqlgo.New(conn)

	// Create the tables if it does not exist.
	wisQueries.CreateFilesTable(ctx)
	if err != nil {
		return nil, eris.Wrap(err, "Error while creating files table")
	}

	wisQueries.CreateExtensionVector(ctx)
	if err != nil {
		return nil, eris.Wrap(err, "Error while creating extension vector")
	}

	wisQueries.CreateEmbeddingsTable(ctx)
	if err != nil {
		return nil, eris.Wrap(err, "Error while creating embeddings table")
	}

	return conn, nil
}

// Delete old files ticker
func DatabaseCleanup(db *pgx.Conn) {
	DeleteOldFilesHandler := func(db *pgx.Conn) error {
		err := wisQueries.DeleteFileWhereOlderThan(context.Background(), int32(deleteFilesAfterDays))

		if err != nil {
			return eris.Wrap(err, "Error while executing DeleteFileWhereOlderThan query")
		}
		return nil
	}

	time.Sleep(5 * time.Second)

	err := func() error {
		// Trigger cleanup process every hour.
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			fmt.Println("Running cleanup process")
			err := DeleteOldFilesHandler(db)
			if err != nil {
				return eris.Wrap(err, "Error while running cleanup process")
			}

			<-ticker.C
		}
	}()

	ErrorTrace(err)
}
