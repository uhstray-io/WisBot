package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

var (
	serverIp                = os.Getenv("SERVER_IP")
	serverPort              = os.Getenv("SERVER_PORT")
	maxFilesPerUser, _      = strconv.ParseInt(os.Getenv("MAX_FILES_PER_USER"), 10, 64)
	deleteFilesAfterDays, _ = strconv.Atoi(os.Getenv("DELETE_FILES_AFTER_DAYS"))
	maxFileSize, _          = strconv.ParseInt(os.Getenv("MAX_FILE_SIZE"), 10, 64)

	databaseUrl = os.Getenv("DATABASE_URL")

	discordToken = os.Getenv("DISCORD_TOKEN_WISBOT")

	ollamaUrl   = os.Getenv("OLLAMA_URL")
	ollamaModel = os.Getenv("OLLAMA_MODEL")
)

func main() {
	// Initialize the database
	db := StartDatabase()
	defer db.Close(context.Background())

	// Start the database cleanup process
	go DatabaseCleanup(db)

	// Start the LLM
	// go StartLLM()
	go StartEmbeddings()

	go WebServer()
	go StartBot()

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("If in iteractive mode, Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
