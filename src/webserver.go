package main

import (
	"fmt"
	"net/http"
	"time"
	"wisbot/src/httpwis"

	"github.com/alexedwards/scs/v2"
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

func requestStackTrace(next func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := next(w, r)
		if err != nil {
			err = fmt.Errorf("error while executing request: %v: %w", r.URL.Path, err)
		}

		PrintTrace(err)
	}
}

func getRoot(w http.ResponseWriter, r *http.Request) {
	userCount := sessionManager.GetInt(r.Context(), "session")
	// fmt.Println(userCount)

	component := httpwis.RootPage(globalState.Count, userCount)
	component.Render(r.Context(), w)
}

func postRoot(w http.ResponseWriter, r *http.Request) {
	// Update state.
	r.ParseForm()

	// Check to see if the global button was pressed.
	if r.Form.Has("global") {
		globalState.Count++
	}
	if r.Form.Has("session") {
		currentSessionCount := sessionManager.GetInt(r.Context(), "session")
		sessionManager.Put(r.Context(), "session", currentSessionCount+1)
	}

	// Display the form.
	getRoot(w, r)
}

func WebServer() {
	sessionManager = scs.New()
	sessionManager.Lifetime = 24 * time.Hour

	server := http.NewServeMux()

	server.HandleFunc("GET /", requestLogger(getRoot))
	server.HandleFunc("POST /", requestLogger(postRoot))

	server.HandleFunc("GET /id/{id}", requestLogger(getId))
	server.HandleFunc("POST /id/{id}/upload", requestLogger(requestStackTrace(postIdUploadFile)))
	server.HandleFunc("GET /id/{id}/download", requestLogger(requestStackTrace(getIdDownloadFile)))

	server.HandleFunc("GET /llm", requestLogger(getLLM))
	server.HandleFunc("GET /llm/chat", requestLogger(getLLMChat))
	server.HandleFunc("POST /llm/chat", requestLogger(postLLMChat))

	// Serve static files
	server.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Add the middleware.
	muxWithSessionMiddleware := sessionManager.LoadAndSave(server)

	// Start the server.
	fmt.Println("listening on", string(serverPort))
	err := http.ListenAndServe(":"+serverPort, muxWithSessionMiddleware)
	if err != nil {
		err = fmt.Errorf("error while issuing ListenAndServe: %w", err)
	}

	PrintTrace(err)
}

func getLLM(w http.ResponseWriter, r *http.Request) {
	component := httpwis.LlmPage()
	component.Render(r.Context(), w)
}

func getLLMChat(w http.ResponseWriter, r *http.Request) {
	component := httpwis.ChatPage()
	component.Render(r.Context(), w)
}

func postLLMChat(w http.ResponseWriter, r *http.Request) {

	fmt.Println("Started LLM chat")

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	question := r.FormValue("question")
	if question == "" {
		http.Error(w, "Question cannot be empty", http.StatusBadRequest)
		return
	}

	fmt.Println("User question:", question)

	// Render the user's message immediately
	userMsg := httpwis.UserMessage(question)
	userMsg.Render(r.Context(), w)

	// Send the question to the LLM
	InputChannel <- question

	// Wait for the response
	response := <-OutputChannel

	// Render the bot's response
	botMsg := httpwis.BotMessage(response)
	botMsg.Render(r.Context(), w)
}
