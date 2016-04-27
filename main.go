package seaeye

import (
	"log"
	"os"
	"os/signal"
)

// LogFilePath defines the path to the build log stdout and stderr.
const LogFilePath = "output.txt"

// CommitStatus specifies the Github API reponse for commit statuses.
type CommitStatus struct {
	Repo        string `json:"-"`
	Rev         string `json:"-"`
	State       string `json:"state"`
	Description string `json:"description,omitempty"`
	TargetURL   string `json:"target_url,omitempty"`
	Context     string `json:"context,omitempty"`
}

// SubEvent specifies a Hookbot event.
type SubEvent struct {
	Branch string
	Repo   string
	SHA    string
	Type   string
	Who    string
}

// Start is the main entrypoint for the server.
func Start(conf *Config) {
	// Start hookbot subscriber
	msgs := spawnSubscriber(conf.HookbotEndpoint)
	events := spawnEventHandler(msgs)

	// Start integrator (checkout, build, test)
	statuses := spawnIntegrator(events, baseDir)

	ghREST := githubRestPOST(user, token)
	spawnGithubNotifier(statuses, ghREST, urlPrefix)

	spawnServer(port, baseDir, events)

	keepAlive()
}

func keepAlive() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	log.Println("Info: Started")
	<-c
	log.Println("Info: Stopped")
}
