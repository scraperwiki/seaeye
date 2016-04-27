package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/scraperwiki/git-prep-directory"
)

// DeletedBranchHash defines the magic git commit hash for a deleted branch.
const DeletedBranchHash = "0000000000000000000000000000000000000000"

func spawnIntegrator(events <-chan SubEvent, baseDir string) <-chan CommitStatus {
	statuses := make(chan CommitStatus)

	go func() {
		for event := range events {
			log.Println("Debug: Event:", event.Repo, event.SHA)

			if event.Type != "push" || event.SHA == DeletedBranchHash {
				continue
			}

			go runPipeline(baseDir, event.Repo, event.SHA, statuses)
		}
	}()

	return statuses
}

func runPipeline(baseDir string, repo string, rev string, statuses chan<- CommitStatus) {
	logPath := path.Join(baseDir, "log", rev, LogFilePath)

	status := func(state string, description string) CommitStatus {
		return CommitStatus{
			Repo:        repo,
			Rev:         rev,
			State:       state,
			Description: description,
			TargetURL:   fmt.Sprintf("/status/%s", rev),
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
	repoURL := fmt.Sprintf("git@github.com:%s", repo)
	log.Println("Info: Stage Checkout start:", checkoutDir, repoURL, rev)
	buildDir, err := stageCheckout(checkoutDir, repoURL, rev)
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
