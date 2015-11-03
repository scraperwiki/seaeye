// seaeye subscribes to branch pushes from Github on Hookbot and runs continuous
// integrations reporting result back to the commit status on Github.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"

	"github.com/codegangsta/cli"
	"github.com/gorilla/mux"
	"github.com/scraperwiki/git-prep-directory"
	"github.com/scraperwiki/hookbot/pkg/listen"

	"golang.org/x/crypto/ssh/terminal"
)

const LOG_FILE = "output.txt"

type SubEvent struct {
	Branch string
	Repo   string
	SHA    string
	Type   string
	Who    string
}

type CommitStatus struct {
	Repo        string `json:"-"`
	Rev         string `json:"-"`
	State       string `json:"state"`
	Description string `json:"description,omitempty"`
	TargetUrl   string `json:"target_url,omitempty"`
	Context     string `json:"context,omitempty"`
}

type Notifier func(string, interface{}) error

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

func init() {
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		log.SetPrefix("\x1b[34;1mseaeye\x1b[0m ")
	} else {
		log.SetPrefix("seaeye ")
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "seaeye"
	app.Usage = "CI server integrating Github and Hookbot."
	app.Version = "1.0"
	app.Action = ActionMain
	app.RunAndExitOnError()
}

func ActionMain(c *cli.Context) {
	port := assertEnv("PORT", "")
	endpoint := assertEnv("HOOKBOT_SUB_ENDPOINT", "")
	user := assertEnv("GITHUB_USER", "")
	token := assertEnv("GITHUB_TOKEN", "")
	authority := assertEnv("CI_URL_AUTHORITY", "localhost")
	urlPrefix := fmt.Sprintf("http://%s:%s", authority, port)

	baseDir, err := os.Getwd()
	if err != nil {
		log.Fatalln("Error:", err)
	}

	// TODO: Put integrator in own file?
	msgs := spawnSubscriber(endpoint)
	events := spawnEventHandler(msgs)
	statuses := spawnIntegrator(events, baseDir)
	ghREST := githubRestPOST(user, token)
	spawnGithubNotifier(statuses, ghREST, urlPrefix)

	// TODO: Put server part in own file?
	spawnServer(port, baseDir, events)

	keepAlive()
}

func assertEnv(key string, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		if fallback != "" {
			return fallback
		}
		log.Fatalln("Error:", key, "not set")
	}
	return val
}

func keepAlive() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	log.Println("Info: Started")
	<-c
	log.Println("Info: Stopped")
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
	outPath := path.Join(baseDir, "log", commit, LOG_FILE)
	http.ServeFile(w, r, outPath)
}

func spawnSubscriber(endpoint string) <-chan []byte {
	msgs, errs := listen.RetryingWatch(endpoint, http.Header{}, nil)
	go errorHandler(errs)

	return msgs
}

func errorHandler(errs <-chan error) {
	for err := range errs {
		log.Println("Warn: Subscription error:", err)
	}
}

func spawnEventHandler(msgs <-chan []byte) chan SubEvent {
	events := make(chan SubEvent)

	go func() {
		for msg := range msgs {
			// HACK: Recursive topic messages are of format '{path}\x00{data}'.
			parts := bytes.Split(msg, []byte{'\x00'})
			if len(parts) == 2 {
				msg = parts[1]
			}

			var event SubEvent
			if err := json.Unmarshal(msg, &event); err != nil {
				log.Printf("Warn: Event error: %v: %v", err, string(msg[:]))
				continue
			}

			events <- event
		}
	}()

	return events
}

func spawnIntegrator(events <-chan SubEvent, baseDir string) <-chan CommitStatus {
	statuses := make(chan CommitStatus)

	go func() {
		const DELETED_BRANCH_HASH = "0000000000000000000000000000000000000000"

		for event := range events {
			log.Println("Debug: Event:", event.Repo, event.SHA)

			if event.Type != "push" || event.SHA == DELETED_BRANCH_HASH {
				continue
			}

			go runPipeline(baseDir, event.Repo, event.SHA, statuses)
		}
	}()

	return statuses
}

func runPipeline(baseDir string, repo string, rev string, statuses chan<- CommitStatus) {
	logPath := path.Join(baseDir, "log", rev, LOG_FILE)

	status := func(state string, description string) CommitStatus {
		return CommitStatus{
			Repo:        repo,
			Rev:         rev,
			State:       state,
			Description: description,
			TargetUrl:   fmt.Sprintf("/status/%s", rev),
		}
	}

	statuses <- status("pending", "Starting...")

	if err := os.MkdirAll(path.Dir(logPath), 0755); err != nil {
		log.Println("Error: Stage Prepare failed:", rev, err)
		statuses <- status("failure", "Stage Prepare failed")
		return
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		log.Println("Error: Stage Prepare failed:", rev, err)
		statuses <- status("failure", "Stage Prepare failed")
		return
	}
	defer logFile.Close()

	checkoutDir := path.Join(baseDir, "src", repo)
	repoUrl := fmt.Sprintf("git@github.com:%s", repo)
	log.Println("Info: Stage Checkout start:", checkoutDir, repoUrl, rev)
	buildDir, err := stageCheckout(checkoutDir, repoUrl, rev)
	if err != nil {
		log.Println("Error: Stage Checkout failed:", rev, err)
		statuses <- status("error", "Stage Checkout failed")
		return
	}
	log.Println("Info: Stage Checkout succeeded:", rev, buildDir.Dir)

	defer func() {
		log.Println("Info: Stage Cleanup start:", rev)
		buildDir.Cleanup()
		log.Println("Info: Stage Cleanup finish:", rev)
	}()

	log.Println("Info: Stage BuildAndTest start:", rev, logFile.Name)
	err = stageBuildAndTest(buildDir.Dir, logFile)
	if err != nil {
		log.Println("Error: Stage BuildAndTest failed:", rev, err)
		if _, ok := err.(*exec.ExitError); ok {
			statuses <- status("failure", "Stage BuildAndTest failed")
		} else {
			statuses <- status("error", "Stage BuildAndTest failed")
		}
		return
	}
	log.Println("Info: Stage BuildAndTest succeeded:", rev)
	statuses <- status("success", "Stage BuildAndTest succeeded")
}

func stageCheckout(checkoutDir string, url string, rev string) (*git.BuildDirectory, error) {
	log.Println("Info: Running git-prep-directory:", checkoutDir, url, rev)
	return git.PrepBuildDirectory(checkoutDir, url, rev)
}

func stageBuildAndTest(buildDir string, logFile *os.File) error {
	cmd := exec.Command("make", "ci")
	cmd.Dir = buildDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	log.Println("Info: Running command:", cmd.Dir, cmd.Path, cmd.Args)
	return cmd.Run()
}

func spawnGithubNotifier(statuses <-chan CommitStatus, notify Notifier, urlPrefix string) {
	go func() {
		const GH_LINK string = "https://api.github.com/repos/%s/statuses/%s"

		for status := range statuses {
			status.Context = "ci"
			status.TargetUrl = fmt.Sprint(urlPrefix, status.TargetUrl)
			url := fmt.Sprintf(GH_LINK, status.Repo, status.Rev)
			log.Println("Info: Notify Github:", url, status)
			if err := notify(url, status); err != nil {
				log.Println("Error:", err)
			}
		}
	}()
}

func githubRestPOST(user string, token string) Notifier {
	return func(url string, payload interface{}) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("POST", url, bytes.NewReader(data))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		// https://developer.github.com/v3/#Authentication
		//req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
		req.SetBasicAuth(user, token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			body, _ := ioutil.ReadAll(resp.Body)
			return fmt.Errorf("%s: %v", resp.Status, body)
		}

		return nil
	}
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
