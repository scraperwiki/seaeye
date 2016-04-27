package seaeye

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"mime"
	"net/http"
	"path"

	"github.com/gorilla/mux"
)

// GithubPushEvent represents a minimal subset of actual webhook push event from
// Github.
type GithubPushEvent struct {
	After      string           `json:"after"`
	Repository GithubRepository `json:"repository"`
}

// GithubRepository is part of the GithubPushEvent struct.
type GithubRepository struct {
	FullName string `json:"full_name"`
}

func spawnServer(port string, baseDir string, events chan SubEvent) {
	logHandler := func(w http.ResponseWriter, r *http.Request) {
		logHandler(w, r, baseDir)
	}

	runHandler := func(w http.ResponseWriter, r *http.Request) {
		runHandler(w, r, events)
	}

	r := mux.NewRouter()
	r.HandleFunc("/status/{commit}", logHandler).Methods("GET")
	r.HandleFunc("/status", runHandler).Methods("PUT", "POST")
	// TODO: Add index route to allow discovery?
	http.Handle("/", r)

	go func() {
		log.Printf("Info: Listening on :%s", port)
		log.Fatalln(http.ListenAndServe(":"+port, nil))
	}()
}

func runHandler(w http.ResponseWriter, r *http.Request, events chan SubEvent) {
	w.Header().Set("Content-Type", "application/json")

	contentType := r.Header.Get("Content-Type")
	contentType, _, _ = mime.ParseMediaType(contentType)
	if contentType != "application/json" && contentType != "application/x-www-form-urlencoded" {
		log.Println("Error: Unsupported Content-Type:", contentType)
		replyUnsupportedMediaType(w, "Unsupported Content-Type: "+contentType)
		return
	}

	var event GithubPushEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Println("Error:", err)
		replyBadRequest(w, err.Error())
		return
	}

	if event.After == "" || event.Repository.FullName == "" {
		log.Println("Error: Invalid push event:", event)
		replyBadRequest(w, "Invalid push event")
		return
	}

	log.Println("Info: Accepted Github webhook:", event)
	replyOK(w)

	events <- SubEvent{
		Type: "push",
		Repo: event.Repository.FullName,
		SHA:  event.After,
	}
}

func logHandler(w http.ResponseWriter, r *http.Request, baseDir string) {
	vars := mux.Vars(r)
	commit := vars["commit"]

	// Note: Tighly coupled with the logPath in runPipeline.
	outPath := path.Join(baseDir, "log", commit, LogFilePath)
	http.ServeFile(w, r, outPath)
}

func replyOK(w http.ResponseWriter) {
	w.Write([]byte(`{"status": "ok"}`))
}

func replyBadRequest(w http.ResponseWriter, reason string) {
	http.Error(w, jsonErrorReply(reason), http.StatusBadRequest)
}

func replyUnsupportedMediaType(w http.ResponseWriter, reason string) {
	http.Error(w, jsonErrorReply(reason), http.StatusUnsupportedMediaType)
}

func jsonErrorReply(reason string) string {
	return fmt.Sprintf(`{"status": "error", "reason": "%s"}`, html.EscapeString(reason))
}
