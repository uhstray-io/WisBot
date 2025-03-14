package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
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
	// Create a root context with cancellation
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Initialize OpenTelemetry
	shutdown, err := setupOTelSDK(ctx)
	if err != nil {
		fmt.Printf("Error setting up OpenTelemetry: %v\n", err)
	}
	// Ensure cleanup at the end
	defer func() {
		err := shutdown(ctx)
		if err != nil {
			fmt.Printf("Error shutting down OpenTelemetry: %v\n", err)
		}
	}()

	// Initialize the database
	db, err := StartDatabase(ctx)
	if err != nil {
		PrintTrace(err)
	}
	defer db.Close(context.Background())

	// Start the database cleanup process
	go StartDatabaseCleanup(ctx, db)

	// Start the LLM
	go StartLLM(ctx)

	// Start the web server
	go WebServer(ctx)
	go StartBot(ctx)

	// Wait for context cancellation
	<-ctx.Done()
}
