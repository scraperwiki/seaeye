package main

import (
	"log"
	"os"
	"os/signal"

	"golang.org/x/crypto/ssh/terminal"
)

const LOG_FILE = "output.txt"

type CommitStatus struct {
	Repo        string `json:"-"`
	Rev         string `json:"-"`
	State       string `json:"state"`
	Description string `json:"description,omitempty"`
	TargetUrl   string `json:"target_url,omitempty"`
	Context     string `json:"context,omitempty"`
}

type SubEvent struct {
	Branch string
	Repo   string
	SHA    string
	Type   string
	Who    string
}

func init() {
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		log.SetPrefix("\x1b[34;1mseaeye\x1b[0m ")
	} else {
		log.SetPrefix("seaeye ")
	}
}

func Run(port string, baseDir string, user string, token string, urlPrefix string, endpoint string) {
	msgs := spawnSubscriber(endpoint)
	events := spawnEventHandler(msgs)
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
