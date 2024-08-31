package main

import (
	"context"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"
)

var db *sql.DB

type File struct {
	Id       string
	Uploaded bool

	CreatedAt       string
	DiscordUsername string

	Name string // Name can not be null in sqlite, it should always be set to "" if not used.
	Data []byte
	Size int
}

func initDatabase(dbPath string) (*sql.DB, error) {
	var err error

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS File (
			Id TEXT PRIMARY KEY,
			Uploaded BOOLEAN NOT NULL,

			CreatedAt TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			DiscordUsername TEXT NOT NULL,

			Name TEXT NOT NULL, 
			Data BLOB,
			Size INTEGER
		)`,
	)

	if err != nil {
		return nil, err
	}
	return db, nil
}
