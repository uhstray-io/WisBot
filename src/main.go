package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	// _ "github.com/mattn/go-sqlite3"
	// _ "modernc.org/sqlite"
)

var (
	maxFilesPerUser, _ = strconv.ParseInt(os.Getenv("MAX_FILES_PER_USER"), 10, 64)
	serverPort         = os.Getenv("SERVER_PORT")
	serverIp           = os.Getenv("SERVER_IP")

	deleteFilesAfterDays, _ = strconv.Atoi(os.Getenv("DELETE_FILES_AFTER_DAYS"))
)

func main() {
	// Initialize the database
	db := StartDatabase()
	defer db.Close(context.Background())

	// Start the database cleanup process
	go DatabaseCleanup(db)

	// Start the LLM
	go StartLLM()

	go StartBot()
	go WebServer()

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("If in iteractive mode, Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
