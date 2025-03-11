package main

import (
	"fmt"
	"net/http"
	"time"
	"wisbot/src/httpwis"

	"github.com/alexedwards/scs/v2"
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
		fmt.Printf("Request: `%s %s` %s", r.Method, r.URL.Path, "\n")
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

	server.HandleFunc("GET /llm/", requestLogger(getLLM))

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
