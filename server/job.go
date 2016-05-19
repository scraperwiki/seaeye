package seaeye

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
)

const (
	// LogBaseDir defines the base directory to log files.
	logBaseDir = "log"
	// FetchBaseDir defines the base directory to any fetched source files.
	fetchBaseDir = "src"
)

// Job is responsible for an describes all necessary modules to execute a job.
type Job struct {
	Config   *Config         // ...need URLPrefix for targetURL
	Fetcher  *GithubFetcher  // ...want to clone a git repo before testing.
	Logger   *FileLogger     // ...want to provide a persistent durable log that can be accessed via REST endpoint.
	Manifest *Manifest       // ...need environment and test script.
	Notifier *GithubNotifier // ...talk the notifier from the executor.
	Trigger  *HookbotTrigger // ...because we need to start the trigger hook from web.
}

// Execute executes a given task: 1. Setup, 2. Run (2a. Fetch, 2b. Test).
func (j *Job) Execute(target *GithubPushEvent) error {
	if err := j.Setup(target); err != nil {
		return err
	}
	defer j.Logger.outFile.Close()

	j.Logger.Println("Info: [executor] Running")
	if err := j.Run(); err != nil {
		j.Logger.Printf("Error: [executor] Run failed: %v", err)
		return err
	}

	return nil
}

// Setup sets up notifier and logger for execution run.
func (j *Job) Setup(target *GithubPushEvent) error {
	repoName := target.Repository.FullName
	repoURL := target.Repository.SSHURL
	rev := target.After

	if j.Logger == nil {
		taskID := taskID(j.Manifest.ID, rev)
		logFilePath := LogFilePath(j.Manifest.ID, rev)
		logger, err := NewFileLogger(logFilePath, taskID+" ", log.LstdFlags)
		if err != nil {
			return err
		}
		j.Logger = logger
		log.Printf("Info: [executor] Created logger: %s", j.Logger.outFile.Name())
	}

	if j.Fetcher == nil {
		f := &GithubFetcher{
			BaseDir:  path.Join(fetchBaseDir, repoName),
			RepoName: repoName,
			RepoURL:  repoURL,
			Rev:      rev,
		}
		j.Fetcher = f
	}

	if j.Notifier != nil {
		targetURL := fmt.Sprintf("%s/api/%s/status/%s", j.Config.URLPrefix, j.Manifest.ID, rev)
		j.Notifier.SetContext(repoName, rev, targetURL)
	}

	return nil
}

// Run executes the pipeline.
func (j *Job) Run() error {
	_ = j.Notifier.Notify("pending", "Starting...")

	// FIXME(uwe): Either fetch into a docker container already running, or
	// fetch first outside container and then copy all files into the container,
	// build it and then run it. Or: Fetch files first outside container, no
	// special built just run a standard container and volume mount src files.
	//
	// Questions to answer:
	//
	// - If fetch from within container, how to do caching? Volume mount?
	// - Which docker images to use as base (should be the docker:latest images, but likely need docker, docker-compose, go, git, etc...): Dockerfile.base?
	// - How to know which tools have to be in the container?
	// - Should getting tools be specified in the manifest as docker run commands?

	// Fetch
	j.Logger.Printf("Info: [executor] Fetching started")
	_ = j.Notifier.Notify("pending", "Stage Fetching started")
	err := j.Fetcher.Fetch()
	if err != nil {
		j.Logger.Printf("Error: [executor] Fetching failed: %v", err)
		_ = j.Notifier.Notify("error", "Stage Fetching failed")
		return err
	}
	j.Logger.Printf("Info: [executor] Fetching succeeded")
	defer j.Fetcher.Cleanup()

	// Test
	j.Logger.Printf("Info: [executor] Testing started")
	if err := j.Test(j.Fetcher.CheckoutDir()); err != nil {
		j.Logger.Printf("Error: [executor] Testing failed: %v", err)
		if _, ok := err.(*exec.ExitError); ok {
			_ = j.Notifier.Notify("failure", "Stage Testing failed")
		} else {
			_ = j.Notifier.Notify("error", "Stage Testing failed")
		}
		return err
	}
	j.Logger.Printf("Info: [executor] Testing succeeded")

	// Done
	_ = j.Notifier.Notify("success", "All stages succeeded")
	return nil
}

// Test runs the tests defined in the manifest.
func (j *Job) Test(wd string) error {
	for _, line := range j.Manifest.Test {
		cmd := exec.Command(line[0], line[1:]...)
		cmd.Dir = wd
		cmd.Env = append(os.Environ(), j.Manifest.Environment...)
		cmd.Stdout = j.Logger.outFile
		cmd.Stderr = j.Logger.outFile

		j.Logger.Printf("Info: [executor] Running command: %v (%s)", cmd.Args, cmd.Dir)
		if err := cmd.Run(); err != nil {
			j.Logger.Printf("Info: [executor] Command failed: %v", err)
			return err
		}
	}
	return nil
}

// LogFilePath assembles a log file path from a manifest id and revision..
func LogFilePath(id, rev string) string {
	idDir := strings.Replace(id, "/", "_", -1)
	return path.Join(logBaseDir, idDir, taskID(id, rev), "log.txt")
}

// taskID assembles a task id from a manifest id and revision.
func taskID(id, rev string) string {
	return strings.Replace(path.Join(id, rev), "/", "_", -1)
}
