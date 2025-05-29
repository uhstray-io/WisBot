package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
)

var (
	httpServiceEnabled = os.Getenv("HTTP_SERVICE_ENABLED") == "true"
	httpServerIp       = os.Getenv("HTTP_SERVER_IP")
	httpServerPort     = os.Getenv("HTTP_SERVER_PORT")

	maxFilesPerUser, _      = strconv.ParseInt(os.Getenv("MAX_FILES_PER_USER"), 10, 64)
	deleteFilesAfterDays, _ = strconv.Atoi(os.Getenv("DELETE_FILES_AFTER_DAYS"))
	maxFileSize, _          = strconv.ParseInt(os.Getenv("MAX_FILE_SIZE"), 10, 64)
	databaseUrl             = os.Getenv("DATABASE_URL")

	postgresServiceEnabled = os.Getenv("POSTGRES_SERVICE_ENABLED") == "true"
	// postgresUser           = os.Getenv("POSTGRES_USER")
	// postgresPassword       = os.Getenv("POSTGRES_PASSWORD")
	// postgresDatabase       = os.Getenv("POSTGRES_DB")

	discordServiceEnabled = os.Getenv("DISCORD_SERVICE_ENABLED") == "true"
	discordToken          = os.Getenv("DISCORD_TOKEN_WISBOT")

	ollamaServiceEnabled = os.Getenv("OLLAMA_SERVICE_ENABLED") == "true"
	ollamaUrl            = os.Getenv("OLLAMA_URL")
	ollamaModel          = os.Getenv("OLLAMA_MODEL")

	otelServiceEnabled       = os.Getenv("OTEL_SERVICE_ENABLED") == "true"
	otelExporterOtlpEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	otelServiceName          = os.Getenv("OTEL_SERVICE_NAME")
	otelResourceAttrs        = os.Getenv("OTEL_RESOURCE_ATTRIBUTES")
)

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
