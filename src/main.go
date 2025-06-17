package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	httpServerIp             string
	httpServerPort           string
	maxFilesPerUser          int64
	deleteFilesAfterDays     int
	maxFileSize              int64
	databaseUrl              string
	discordToken             string
	ollamaUrl                string
	ollamaModel              string
	otelExporterOtlpEndpoint string
	otelServiceName          string
	otelResourceAttrs        string
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// HTTP server configuration
	httpServerIp = os.Getenv("HTTP_SERVER_IP")
	httpServerPort = os.Getenv("HTTP_SERVER_PORT")

	// Database configuration
	maxFilesPerUser, _ = strconv.ParseInt(os.Getenv("MAX_FILES_PER_USER"), 10, 64)
	deleteFilesAfterDays, _ = strconv.Atoi(os.Getenv("DELETE_FILES_AFTER_DAYS"))
	maxFileSize, _ = strconv.ParseInt(os.Getenv("MAX_FILE_SIZE"), 10, 64)

	// Database connection
	databaseUrl = os.Getenv("DATABASE_URL")

	// Discord integration
	discordToken = os.Getenv("DISCORD_TOKEN_WISBOT")

	// LLM (Ollama) integration
	ollamaUrl = os.Getenv("OLLAMA_URL")
	ollamaModel = os.Getenv("OLLAMA_MODEL")

	// OpenTelemetry configuration
	otelExporterOtlpEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	otelServiceName = os.Getenv("OTEL_SERVICE_NAME")
	otelResourceAttrs = os.Getenv("OTEL_RESOURCE_ATTRIBUTES")
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go StartOTelService(ctx)
	go StartDatabaseService(ctx)
	go StartLLMService(ctx)
	go StartHTTPService(ctx)
	go StartDiscordService(ctx)

	// Wait for context cancellation
	<-ctx.Done()
}
