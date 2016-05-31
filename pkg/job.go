package seaeye

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

const (
	// LogBaseDir defines the base directory to log files.
	logBaseDir = "logs"
	// FetchBaseDir defines the base directory to any fetched source files.
	fetchBaseDir = "workspace"
)

// Job is responsible for an describes all necessary modules to execute a job.
type Job struct {
	Config   *Config         // ...to prefix targetURL with BaseURL.
	Fetcher  *GithubFetcher  // ...to clone git repo.
	ID       string          // ...to identify for logs.
	Logger   *FileLogger     // ...to accessed persistent and durable logs via REST endpoint.
	Manifest *Manifest       // ...to provide envvars and test instructions.
	Notifier *GithubNotifier // ...to update commmit statuses.
}

// Execute executes a given task: 1. Setup, 2. Run (2a. Fetch, 2b. Test).
func (j *Job) Execute(s *Source) error {
	if err := j.setup(s); err != nil {
		return err
	}
	defer j.Logger.outFile.Close()

	j.Logger.Println("[I][executor] Running")
	if err := j.run(); err != nil {
		j.Logger.Printf("[E][executor] Run failed: %v", err)
		return err
	}

	return nil
}

// Setup ensures that all relevant job parts are configured and instatiated.
func (j *Job) setup(s *Source) error {
	if j.ID == "" {
		j.ID = url.QueryEscape(path.Join(s.Owner, s.Repo))
	}

	if j.Logger == nil {
		logFilePath, err := filepath.Abs(LogFilePath(j.ID, s.Rev))
		if err != nil {
			return err
		}
		prefix := log.Prefix() + j.ID + " "
		logger, err := NewFileLogger(logFilePath, prefix, log.LstdFlags)
		if err != nil {
			return err
		}
		j.Logger = logger
		log.Printf("[I][executor] Created logger: %s", j.Logger.outFile.Name())
	}

	if j.Fetcher == nil {
		f := &GithubFetcher{
			BaseDir: path.Join(fetchBaseDir, s.Owner, s.Repo),
			Source:  s,
		}
		j.Fetcher = f
	}

	if j.Notifier == nil {
		c := NewOAuthGithubClient(j.Config.GithubToken)
		t := j.Config.BaseURL + fmt.Sprintf("/jobs/%s/status/%s", j.ID, s.Rev)
		n := &GithubNotifier{
			Client:    c,
			Source:    s,
			TargetURL: t,
		}
		j.Notifier = n
	}

	return nil
}

// Run executes the pipeline.
func (j *Job) run() error {
	_ = j.Notifier.Notify("pending", "Starting...")

	// TODO(uwe): Either fetch into a docker container already running, or
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
	j.Logger.Printf("[I][executor] Fetching started")
	_ = j.Notifier.Notify("pending", "Stage Fetching started")
	err := j.Fetcher.Fetch()
	if err != nil {
		j.Logger.Printf("[E][executor] Fetching failed: %v", err)
		_ = j.Notifier.Notify("error", "Stage Fetching failed")
		return err
	}
	j.Logger.Printf("[I][executor] Fetching succeeded")

	// Defer Cleanup
	//defer j.Fetcher.Cleanup()

	// Test
	j.Logger.Printf("[I][executor] Testing started")
	if err := j.Test(j.Fetcher.CheckoutDir()); err != nil {
		j.Logger.Printf("[E][executor] Testing failed: %v", err)
		if _, ok := err.(*exec.ExitError); ok {
			_ = j.Notifier.Notify("failure", "Stage Testing failed")
		} else {
			_ = j.Notifier.Notify("error", "Stage Testing failed")
		}
		return err
	}
	j.Logger.Printf("[I][executor] Testing succeeded")

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

		j.Logger.Printf("[I][executor] Running command: %v (%s)", cmd.Args, cmd.Dir)
		if err := cmd.Run(); err != nil {
			j.Logger.Printf("[I][executor] Command failed: %v", err)
			return err
		}
	}
	return nil
}

// LogFilePath assembles a log file path from a job id and revision.
func LogFilePath(jobID, rev string) string {
	saneID := strings.Replace(jobID, "/", "_", -1) // e.g.: scraperwiki/foo
	saneRev := strings.Replace(rev, "/", "_", -1)  // e.g.: refs/origin/master
	return path.Join(logBaseDir, saneID, saneRev, "log.txt")
}
