// seaeye subscribes to branch pushes from Github on Hookbot and runs continuous
// integrations reporting result back to the commit status on Github.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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
	Ref         string `json:"-"`
	State       string `json:"state"`
	Description string `json:"description,omitempty"`
	TargetUrl   string `json:"target_url,omitempty"`
	Context     string `json:"context,omitempty"`
}

type Notifier func(string, interface{}) error

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
	urlPrefix := fmt.Sprintf("https://%s:%s/status", authority, port)

	baseDir, err := os.Getwd()
	if err != nil {
		log.Fatalln("Error:", err)
	}

	// TODO: Put server part in own file?
	spawnServer(port, baseDir)

	// TODO: Put integrator in own file?
	msgs := spawnSubscriber(endpoint)
	events := spawnEventHandler(msgs)
	statuses := spawnIntegrator(events, baseDir)
	ghREST := githubRestPOST(user, token)
	spawnGithubNotifier(statuses, ghREST, urlPrefix)

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

func spawnServer(port string, baseDir string) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		logHandler(w, r, baseDir)
	}

	r := mux.NewRouter()
	// TODO: Add index route to allow discovery?
	r.HandleFunc("/status/{commit}", handler).Methods("GET")
	http.Handle("/", r)

	go func() {
		log.Printf("Info: Listening on :%s", port)
		log.Fatalln(http.ListenAndServe(":"+port, nil))
	}()
}

func logHandler(w http.ResponseWriter, r *http.Request, baseDir string) {
	vars := mux.Vars(r)
	commit := vars["commit"]

	// TODO: Remove git-prep-directory logic
	if len(commit) < 10 {
		http.NotFound(w, r)
		return
	}

	// TODO: Remove git-prep-directory logic
	outPath := path.Join(baseDir, "src", "c", commit[:10], LOG_FILE)
	http.ServeFile(w, r, outPath)
}

func spawnSubscriber(endpoint string) <-chan []byte {
	msgs, errs := listen.RetryingWatch(endpoint, http.Header{}, nil)
	go errorHandler(errs)

	return msgs
}

func errorHandler(errs <-chan error) {
	for err := range errs {
		log.Println("Warn:", err)
	}
}

func spawnEventHandler(msgs <-chan []byte) <-chan SubEvent {
	events := make(chan SubEvent)

	go func() {
		for msg := range msgs {
			var event SubEvent
			if err := json.Unmarshal(msg, &event); err != nil {
				log.Println("Warn:", err)
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

func runPipeline(baseDir string, repo string, ref string, statuses chan<- CommitStatus) {
	status := func(state string, description string) CommitStatus {
		return CommitStatus{
			Repo:        repo,
			Ref:         ref,
			State:       state,
			Description: description,
		}
	}

	statuses <- status("pending", "Starting...")

	checkoutDir := path.Join(baseDir, "src", repo)
	repoUrl := fmt.Sprintf("git@github.com:%s", repo)
	log.Println("Info: Stage Checkout start:", checkoutDir, repoUrl, ref)
	buildDir, err := stageCheckout(checkoutDir, repoUrl, ref)
	if err != nil {
		log.Println("Error: Stage Checkout failed:", ref, err)
		statuses <- status("error", "Stage Checkout failed")
		return
	}
	log.Println("Info: Stage Checkout succeeded:", ref, buildDir.Dir)

	defer func() {
		log.Println("Info: Stage Cleanup start:", ref)
		buildDir.Cleanup()
		log.Println("Info: Stage Cleanup finish:", ref)
	}()

	log.Println("Info: Stage BuildAndTest start:", ref)
	err = stageBuildAndTest(buildDirectory.Dir)
	if err != nil {
		log.Println("Error: Stage BuildAndTest failed:", ref, err)
		if _, ok := err.(*exec.ExitError); ok {
			statuses <- status("failure", "Stage BuildAndTest failed")
		} else {
			statuses <- status("error", "Stage BuildAndTest failed")
		}
		return
	}
	log.Println("Info: Stage BuildAndTest succeeded:", ref)
	statuses <- status("success", "Stage BuildAndTest succeeded")
}

func stageCheckout(checkoutDir string, url string, ref string) (*git.BuildDirectory, error) {
	log.Println("Info: Running git-prep-directory:", checkoutDir, url, ref)
	return git.PrepBuildDirectory(checkoutDir, url, ref)
}

func stageBuildAndTest(workspaceDir string) error {
	outfile, err := os.Create(path.Join(workspaceDir, LOG_FILE))
	if err != nil {
		return err
	}
	defer outfile.Close()

	cmd := exec.Command("make", "ci")
	cmd.Dir = workspaceDir
	cmd.Stdout = outfile
	cmd.Stderr = outfile

	log.Println("Info: Running command:", cmd.Dir, cmd.Path, cmd.Args)
	return cmd.Run()
}

func spawnGithubNotifier(statuses <-chan CommitStatus, notify Notifier, urlPrefix string) {
	go func() {
		const GH_LINK string = "https://api.github.com/repos/%s/statuses/%s"

		for status := range statuses {
			status.Context = "ci"
			status.TargetUrl = fmt.Sprintf("%s/%s", urlPrefix, status.Ref)
			url := fmt.Sprintf(GH_LINK, status.Repo, status.Ref)
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
