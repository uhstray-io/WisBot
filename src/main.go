package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"
)

func main() {
	// Load configuration
	LoadConfig("config.yaml")

	// Initialize the database
	db, err := StartDatabase()
	if err != nil {
		fmt.Println("Error initializing database:", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	DeleteOldFiles(db)
	StartLLM()

	go StartBot()
	go WebServer()

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
