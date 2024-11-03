package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"

	"github.com/jackc/pgx/v5"
)

var db *pgx.Conn

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

func StartDatabase() (*pgx.Conn, error) {
	conn, err1 := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err1 != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err1)
		os.Exit(1)
	}
	// defer conn.Close(context.Background())

	_, err2 := conn.Exec(context.Background(),
		`CREATE TABLE IF NOT EXISTS File (
			Id TEXT PRIMARY KEY,
			Uploaded BOOLEAN NOT NULL,
			CreatedAt TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			DiscordUsername TEXT NOT NULL,
			Name TEXT NOT NULL, 
			Data BYTEA,
			Size INTEGER,
			Downloads INTEGER DEFAULT 0
		)`,
	)

	if err2 != nil {
		fmt.Println("Error creating File table.", err2)
		return nil, err2
	}

	db = conn

	return conn, nil
}

// Delete old files ticker
func DeleteOldFiles(db *pgx.Conn) {

	// Delete files after specified time.
	// DeleteOldFilesHandler := func(db *pgx.Conn) {
	// 	days, _ := strconv.Atoi(os.Getenv("DELETE_FILES_AFTER_DAYS"))
	// 	seconds := 60 * 60 * 24 * days
	// 	secondsString := fmt.Sprintf("-%d seconds", seconds)

	// 	_, err := db.Exec(context.Background(), "DELETE FROM File WHERE CreatedAt < datetime('now', ?)", secondsString)

	// 	if err != nil {
	// 		fmt.Println("Error deleting old files.", err)
	// 	}
	// }

	// time.Sleep(5 * time.Second)

	// go func() {
	// 	// Trigger cleanup process every hour.
	// 	ticker := time.NewTicker(1 * time.Hour)
	// 	defer ticker.Stop()

	// 	for {
	// 		DeleteOldFilesHandler(db)
	// 		<-ticker.C
	// 	}
	// }()
}
