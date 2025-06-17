package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"
	"wisbot/src/sqlc"
	"wisbot/src/templ"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/fiber/v2/middleware/session"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
)

var globalState GlobalState
var store *session.Store

type GlobalState struct {
	Count int
}

func StartHTTPService(ctx context.Context) {
	store = session.New(session.Config{Expiration: 24 * time.Hour})

	app := fiber.New()

	// Middleware

	// Add security headers
	app.Use(helmet.New())

	// Health checks
	app.Use(healthcheck.New(healthcheck.Config{ // / /live and /ready endpoints
		LivenessProbe:     func(c *fiber.Ctx) bool { return true },
		LivenessEndpoint:  "/live",
		ReadinessEndpoint: "/ready",
	}))

	// Rate limiting
	app.Use(limiter.New(limiter.Config{
		Max:               20,
		Expiration:        30 * time.Second,
		LimiterMiddleware: limiter.SlidingWindow{},
	}))

	// Performance profiling
	app.Use(pprof.New()) // /debug/pprof endpoints

	// Request ID
	app.Use(requestid.New())

	// Monitoring endpoint
	app.Get("/metrics", monitor.New()) // /metrics endpoint

	// Routes

	// Direct Fiber routes for session-dependent endpoints
	app.Get("/", getRoot)
	app.Post("/", postRoot)

	app.Get("/id/:id", getId)
	app.Get("/id/:id/download", getIdDownloadFile)
	app.Post("/id/:id/upload", postIdUploadFile)

	app.Get("/llm", getLLM)
	app.Get("/llm/chat", getLLMChat)
	app.Post("/llm/chat", postLLMChat)

	// Keep Huma API available for other endpoints if needed in the future
	_ = humafiber.New(app, huma.DefaultConfig("WisBot API", "0.0.1"))

	// Start the server
	LogEvent(ctx, log.SeverityInfo, "Starting HTTP server", attribute.String("port", httpServerPort))

	err := app.Listen(":" + httpServerPort)
	if err != nil {
		LogError(ctx, err, "Error while starting HTTP server")
	}
	PrintTrace(err)
}

// Fiber handlers for session management
func getRoot(c *fiber.Ctx) error {
	ctx := c.Context()
	sess, err := store.Get(c)
	if err != nil {
		return err
	}

	userCount := sess.Get("count")
	if userCount == nil {
		userCount = 0
	}

	var buf bytes.Buffer
	component := templ.RootPage(globalState.Count, userCount.(int))
	err = component.Render(ctx, &buf)
	if err != nil {
		return err
	}

	c.Set("Content-Type", "text/html")
	return c.Send(buf.Bytes())
}

func postRoot(c *fiber.Ctx) error {
	ctx := c.Context()
	sess, err := store.Get(c)
	if err != nil {
		return err
	}

	// Check which button was pressed
	if c.FormValue("global") != "" {
		globalState.Count++
	}

	if c.FormValue("session") != "" {
		currentCount := sess.Get("count")
		if currentCount == nil {
			currentCount = 0
		}
		sess.Set("count", currentCount.(int)+1)
		sess.Save()
	}

	// Render the updated page
	userCount := sess.Get("count")
	if userCount == nil {
		userCount = 0
	}

	var buf bytes.Buffer
	component := templ.RootPage(globalState.Count, userCount.(int))
	err = component.Render(ctx, &buf)
	if err != nil {
		return err
	}

	c.Set("Content-Type", "text/html")
	return c.Send(buf.Bytes())
}

func getLLM(c *fiber.Ctx) error {
	ctx := c.Context()
	_, span := StartSpan(ctx, "getLLM")
	defer span.End()

	var buf bytes.Buffer
	component := templ.LlmPage()
	err := component.Render(ctx, &buf)
	if err != nil {
		return err
	}

	c.Set("Content-Type", "text/html")
	return c.Send(buf.Bytes())
}

func getLLMChat(c *fiber.Ctx) error {
	ctx := c.Context()
	_, span := StartSpan(ctx, "getLLMChat")
	defer span.End()

	var buf bytes.Buffer
	component := templ.ChatPage()
	err := component.Render(ctx, &buf)
	if err != nil {
		return err
	}

	c.Set("Content-Type", "text/html")
	return c.Send(buf.Bytes())
}

func postLLMChat(c *fiber.Ctx) error {
	ctx := c.Context()
	_, span := StartSpan(ctx, "postLLMChat")
	defer span.End()

	LogEvent(ctx, log.SeverityInfo, "Started LLM chat")

	question := c.FormValue("question")
	if question == "" {
		span.SetAttributes(attribute.String("error", "empty_question"))
		return c.Status(fiber.StatusBadRequest).SendString("Question cannot be empty")
	}
	span.SetAttributes(attribute.Int("question.length", len(question)))

	LogEvent(ctx, log.SeverityInfo, "User question received", attribute.String("question", question))

	// Render the user's message immediately
	var userBuf bytes.Buffer
	userMsg := templ.UserMessage(question)
	err := userMsg.Render(ctx, &userBuf)
	if err != nil {
		return err
	}

	// Send the question to the LLM
	InputChannel <- question
	// Wait for the response
	response := <-OutputChannel
	span.SetAttributes(attribute.Int("response.length", len(response)))

	// Render the bot's response
	var botBuf bytes.Buffer
	botMsg := templ.BotMessage(response)
	err = botMsg.Render(ctx, &botBuf)
	if err != nil {
		return err
	}

	// Combine both messages
	var combinedBuf bytes.Buffer
	combinedBuf.Write(userBuf.Bytes())
	combinedBuf.Write(botBuf.Bytes())

	c.Set("Content-Type", "text/html")
	return c.Send(combinedBuf.Bytes())
}

func getId(c *fiber.Ctx) error {
	ctx := c.Context()
	_, span := StartSpan(ctx, "getId")
	defer span.End()

	id := c.Params("id")
	span.SetAttributes(attribute.String("file_id", id))

	// Grab the file where the ID matches.
	queryfile, err := db.GetFileNameAndUploadFromId(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			var buf bytes.Buffer
			component := templ.RootIdPage(nil)
			err := component.Render(ctx, &buf)
			if err != nil {
				return err
			}
			c.Set("Content-Type", "text/html")
			return c.Send(buf.Bytes())
		}
		LogError(ctx, err, "Error while executing GetFileNameAndUploadFromId query")
		return err
	}

	file := &sqlc.File{ID: queryfile.ID, Name: queryfile.Name, Uploaded: queryfile.Uploaded}

	var buf bytes.Buffer
	component := templ.RootIdPage(file)
	err = component.Render(ctx, &buf)
	if err != nil {
		return err
	}

	c.Set("Content-Type", "text/html")

	return c.Send(buf.Bytes())
}

func postIdUploadFile(c *fiber.Ctx) error {
	ctx := c.Context()

	id := c.Params("id")

	// Check if the ID exists and the file has not been uploaded.
	file := &sqlc.File{ID: id}
	id, err := db.GetFileIdWhereIdAndUploadedIsFalse(ctx, id)

	// If the Id exists, and the file has not been uploaded, then continue.
	if err != nil {
		if err == sql.ErrNoRows {
			var buf bytes.Buffer
			templ.UploadFileFormCompleted(nil, false, "File not found.").Render(ctx, &buf)
			c.Set("Content-Type", "text/html")
			return c.Send(buf.Bytes())
		}
		return fmt.Errorf("error while executing GetFileIdWhereIdAndUploadedIsFalse query: %w", err)
	}
	// Handle the file upload - 100MB max file maxFileSize.
	var maxSize int64 = maxFileSize * 1024 * 1024
	LogEvent(ctx, log.SeverityInfo, "File upload configured", attribute.Int64("max_file_size_bytes", maxSize))

	fileHeader, err := c.FormFile("file")
	if err != nil {
		var buf bytes.Buffer
		templ.UploadFileFormCompleted(file, false, "Unable to read file.").Render(ctx, &buf)
		c.Set("Content-Type", "text/html")
		return c.Send(buf.Bytes())
	}

	if fileHeader.Size > maxSize {
		var buf bytes.Buffer
		templ.UploadFileFormCompleted(file, false, "File too large.").Render(ctx, &buf)
		c.Set("Content-Type", "text/html")
		return c.Send(buf.Bytes())
	}

	// Read file content
	fileObject, err := fileHeader.Open()
	if err != nil {
		var buf bytes.Buffer
		templ.UploadFileFormCompleted(file, false, "Unable to read file.").Render(ctx, &buf)
		c.Set("Content-Type", "text/html")
		return c.Send(buf.Bytes())
	}
	defer fileObject.Close()

	buff, err := io.ReadAll(io.LimitReader(fileObject, int64(maxSize)))
	if err != nil {
		var buf bytes.Buffer
		templ.UploadFileFormCompleted(file, false, "Unable to read file.").Render(ctx, &buf)
		c.Set("Content-Type", "text/html")
		return c.Send(buf.Bytes())
	}

	LogEvent(ctx, log.SeverityInfo, "File upload received",
		attribute.String("file_name", fileHeader.Filename),
		attribute.Int64("file_size_bytes", fileHeader.Size),
		attribute.Int("file_data_length", len(buff)),
		attribute.String("file_header", fmt.Sprintf("%v", fileHeader.Header)),
	)

	// Update the file.
	file.Name = fileHeader.Filename
	file.Data = buff
	file.Uploaded = true

	// Update the file in the database.
	err2 := db.UpdateFileWhereId(ctx,
		sqlc.UpdateFileWhereIdParams{
			ID:       id,
			Name:     file.Name,
			Data:     file.Data,
			Size:     int32(len(buff)),
			Uploaded: file.Uploaded,
		})

	if err2 != nil {
		var buf bytes.Buffer
		templ.UploadFileFormCompleted(file, false, "Unable to update file.").Render(ctx, &buf)
		c.Set("Content-Type", "text/html")
		c.Send(buf.Bytes())
		return fmt.Errorf("error while executing UpdateFileWhereId query: %w", err2)
	}

	var buf bytes.Buffer
	templ.UploadFileFormCompleted(file, true, "").Render(ctx, &buf)
	c.Set("Content-Type", "text/html")
	return c.Send(buf.Bytes())
}

func getIdDownloadFile(c *fiber.Ctx) error {
	ctx := c.Context()

	id := c.Params("id")

	// Grab the file where the ID matches.
	file, err := db.GetFileFromId(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).SendString("file not found")
		}
		return fmt.Errorf("error while executing GetFileFromId query: %w", err)
	}

	c.Set("Content-Disposition", "attachment; filename="+file.Name)
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Length", fmt.Sprintf("%d", file.Size))

	// Increment the download count.
	if err2 := db.UpdateFileDownloadIncrement(ctx, id); err2 != nil {
		return fmt.Errorf("error while executing UpdateFileDownloadIncrement query: %w", err2)
	}

	return c.Send(file.Data)
}
