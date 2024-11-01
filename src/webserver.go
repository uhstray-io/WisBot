package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/v2"
)

var global GlobalState
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

func getRoot(w http.ResponseWriter, r *http.Request) {
	userCount := sessionManager.GetInt(r.Context(), "session")
	// fmt.Println(userCount)

	component := rootPage(global.Count, userCount)
	component.Render(r.Context(), w)
}

func postRoot(w http.ResponseWriter, r *http.Request) {
	// Update state.
	r.ParseForm()

	// Check to see if the global button was pressed.
	if r.Form.Has("global") {
		global.Count++
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
	mux.HandleFunc("POST /id/{id}/upload", requestLogger(postIdUploadFile))
	mux.HandleFunc("GET /id/{id}/download", requestLogger(getIdDownloadFile))

	// Add the middleware.
	muxWithSessionMiddleware := sessionManager.LoadAndSave(mux)

	// Start the server.
	fmt.Println("listening on http://" + os.Getenv("SERVER_IP") + ":" + os.Getenv("SERVER_PORT"))
	if err := http.ListenAndServe(":"+os.Getenv("SERVER_PORT"), muxWithSessionMiddleware); err != nil {
		log.Printf("error listening: %v", err)
	}
}
