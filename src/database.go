package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"
)

var db *sql.DB

type File struct {
	Id              string
	Uploaded        bool
	CreatedAt       string
	DiscordUsername string
	Name            string // Name can not be null in sqlite, it should always be set to "" if not used.
	Data            []byte
	Size            int
	Downloads       int
}

func StartDatabase() (*sql.DB, error) {
	localDB, err1 := sql.Open(config.Database.Type, config.Database.Name)
	if err1 != nil {
		fmt.Println("Error opening database.", err1)
		return nil, err1
	}

	_, err2 := localDB.Exec(
		`CREATE TABLE IF NOT EXISTS File (
			Id TEXT PRIMARY KEY,
			Uploaded BOOLEAN NOT NULL,
			CreatedAt TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			DiscordUsername TEXT NOT NULL,
			Name TEXT NOT NULL, 
			Data BLOB,
			Size INTEGER,
			Downloads INTEGER DEFAULT 0
		)`,
	)

	if err2 != nil {
		fmt.Println("Error creating File table.", err2)
		return nil, err2
	}

	db = localDB

	return localDB, nil
}

// Delete old files ticker
func DeleteOldFiles(db *sql.DB) {

	// Delete files after specified time.
	DeleteOldFilesHandler := func(db *sql.DB) {
		seconds := 60 * 60 * 24 * config.DeleteFilesAfterDays
		secondsString := fmt.Sprintf("-%d seconds", seconds)

		_, err := db.Exec("DELETE FROM File WHERE CreatedAt < datetime('now', ?)", secondsString)
		if err != nil {
			fmt.Println("Error deleting old files.", err)
		}
	}

	go func() {
		// Trigger cleanup process every hour.
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			DeleteOldFilesHandler(db)
			<-ticker.C
		}
	}()
}
