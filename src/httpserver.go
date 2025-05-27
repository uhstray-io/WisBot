package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"wisbot/src/templ"

	"github.com/alexedwards/scs/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

var globalState GlobalState
var sessionManager *scs.SessionManager

type GlobalState struct {
	Count int
}

// requestLogger is a middleware function that logs incoming HTTP requests.
// It takes a http.HandlerFunc as input and returns a new http.HandlerFunc.
// The returned function logs the details of the incoming request and then calls the original handler.
func requestLogger(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// fmt.Printf("Request: `%s %s` %s", r.Method, r.URL.Path, "\n")

		// Get a logger from the global provider
		logger := global.Logger("requestLogger")

		record := log.Record{}
		record.SetTimestamp(time.Now())
		record.SetObservedTimestamp(time.Now())
		record.SetSeverity(log.SeverityInfo)
		record.SetBody(log.StringValue(fmt.Sprintf("Request: `%s %s`", r.Method, r.URL.Path)))
		record.AddAttributes(log.String("method", r.Method))
		record.AddAttributes(log.String("path", r.URL.Path))

		// Log the request details with OpenTelemetry
		logger.Emit(r.Context(), record)

		next(w, r)
	}
}

func requestTracer(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer("wisbot").Start(r.Context(), fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		// span := trace.SpanFromContext(r.Context())
		defer span.End()

		// Add request details as span attributes
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.user_agent", r.UserAgent()),
		)

		// Create new request with the span context
		r = r.WithContext(ctx)

		// Call the actual handler
		next(w, r)
	}
}

func requestStackTrace(next func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := next(w, r)
		if err != nil {
			err = fmt.Errorf("error while executing request: %v: %w", r.URL.Path, err)
		}

		PrintTrace(err)
	}
}

func StartHTTPService(ctx context.Context) {

	if !httpServiceEnabled {
		fmt.Println("HTTP service is disabled. Skipping HTTP server initialization.")
		return
	}

	sessionManager = scs.New()
	sessionManager.Lifetime = 24 * time.Hour

	server := http.NewServeMux()

	server.HandleFunc("GET /", requestTracer(requestLogger(getRoot)))
	server.HandleFunc("POST /", requestTracer(requestLogger(postRoot)))

	server.HandleFunc("GET /id/{id}", requestTracer(requestLogger(getId)))
	server.HandleFunc("POST /id/{id}/upload", requestTracer(requestLogger(requestStackTrace(postIdUploadFile))))
	server.HandleFunc("GET /id/{id}/download", requestTracer(requestLogger(requestStackTrace(getIdDownloadFile))))

	server.HandleFunc("GET /llm", requestTracer(requestLogger(getLLM)))
	server.HandleFunc("GET /llm/chat", requestTracer(requestLogger(getLLMChat)))
	server.HandleFunc("POST /llm/chat", requestTracer(requestLogger(postLLMChat)))

	// Add the middleware.
	muxWithSessionMiddleware := sessionManager.LoadAndSave(server)

	// Start the server.
	fmt.Println("listening on", string(httpServerPort))

	err := http.ListenAndServe(":"+httpServerPort, muxWithSessionMiddleware)
	if err != nil {
		err = fmt.Errorf("error while issuing ListenAndServe: %w", err)
	}

	PrintTrace(err)
}

func getRoot(w http.ResponseWriter, r *http.Request) {
	ctx, span := StartSpan(r.Context(), "getRoot")
	defer span.End()

	userCount := sessionManager.GetInt(ctx, "session")
	span.SetAttributes(attribute.Int("user.count", userCount))

	component := templ.RootPage(globalState.Count, userCount)
	component.Render(ctx, w)
}

func postRoot(w http.ResponseWriter, r *http.Request) {
	ctx, span := StartSpan(r.Context(), "postRoot")
	defer span.End()

	// Update state.
	r.ParseForm()

	// Check to see if the global button was pressed.
	if r.Form.Has("global") {
		globalState.Count++
		span.SetAttributes(attribute.String("action", "global_increment"))
	}
	if r.Form.Has("session") {
		currentSessionCount := sessionManager.GetInt(ctx, "session")
		sessionManager.Put(ctx, "session", currentSessionCount+1)
		span.SetAttributes(attribute.String("action", "session_increment"))
	}

	// Display the form.
	getRoot(w, r.WithContext(ctx))
}

func getLLM(w http.ResponseWriter, r *http.Request) {
	ctx, span := StartSpan(r.Context(), "getLLM")
	defer span.End()

	component := templ.LlmPage()
	component.Render(ctx, w)
}

func getLLMChat(w http.ResponseWriter, r *http.Request) {
	ctx, span := StartSpan(r.Context(), "getLLMChat")
	defer span.End()

	component := templ.ChatPage()
	component.Render(ctx, w)
}

func postLLMChat(w http.ResponseWriter, r *http.Request) {
	ctx, span := StartSpan(r.Context(), "postLLMChat")
	defer span.End()

	fmt.Println("Started LLM chat")

	err := r.ParseForm()
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	question := r.FormValue("question")
	if question == "" {
		span.SetAttributes(attribute.String("error", "empty_question"))
		http.Error(w, "Question cannot be empty", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Int("question.length", len(question)))
	fmt.Println("User question:", question)

	// Render the user's message immediately
	userMsg := templ.UserMessage(question)
	userMsg.Render(ctx, w)

	// Send the question to the LLM
	InputChannel <- question

	// Wait for the response
	response := <-OutputChannel
	span.SetAttributes(attribute.Int("response.length", len(response)))

	// Render the bot's response
	botMsg := templ.BotMessage(response)
	botMsg.Render(ctx, w)
}
