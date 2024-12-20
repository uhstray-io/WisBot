package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/rotisserie/eris"
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
			eris.Wrapf(err, "Error while executing request: %v", r.URL.Path)
		}

		PrintTrace(err)
	}
}

func getRoot(w http.ResponseWriter, r *http.Request) {
	userCount := sessionManager.GetInt(r.Context(), "session")
	// fmt.Println(userCount)

	component := rootPage(globalState.Count, userCount)
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

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", requestLogger(getRoot))
	mux.HandleFunc("POST /", requestLogger(postRoot))
	mux.HandleFunc("GET /id/{id}", requestLogger(getId))
	mux.HandleFunc("POST /id/{id}/upload", requestLogger(requestStackTrace(postIdUploadFile)))
	mux.HandleFunc("GET /id/{id}/download", requestLogger(requestStackTrace(getIdDownloadFile)))

	// Add the middleware.
	muxWithSessionMiddleware := sessionManager.LoadAndSave(mux)

	// Start the server.
	fmt.Println("listening on https://" + serverIp + ":" + serverPort)
	err := http.ListenAndServe(":"+serverPort, muxWithSessionMiddleware)
	if err != nil {
		err = eris.Wrap(err, "Error while issuing ListenAndServe")
	}

	PrintTrace(err)
}
