package seaeye

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/scraperwiki/seaeye/pkg/exec"
	"golang.org/x/net/context"
)

const (
	// LogBaseDir defines the base directory to log files.
	logBaseDir = "logs"
	// FetchBaseDir defines the base directory to any fetched source files.
	fetchBaseDir = "workspace"
	// testExecutionTimeout
	testExecutionTimeout = 1 * time.Hour
)

// Job is responsible for an describes all necessary modules to execute a job.
type Job struct {
	Config   *Config         // ...to prefix targetURL with BaseURL.
	Fetcher  *GithubFetcher  // ...to clone git repo.
	ID       string          // ...to identify for logs.
	Logger   *FileLogger     // ...to accessed persistent and durable logs via REST endpoint.
	Notifier *GithubNotifier // ...to update commmit statuses.
}

// Execute executes a given task: 1. Setup, 2. Run (2a. Fetch, 2b. Test).
func (j *Job) Execute(s *Source) error {
	if err := j.setup(s); err != nil {
		return err
	}
	defer j.Logger.outFile.Close()

	j.Logger.Println("[I][job] Running")
	if err := j.run(); err != nil {
		j.Logger.Printf("[E][job] Run failed: %v", err)
		return err
	}

	return nil
}

// Setup ensures that all relevant job parts are configured and instatiated.
func (j *Job) setup(s *Source) error {
	if j.ID == "" {
		j.ID = sanitizePath(path.Join(s.Owner, s.Repo))
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
		log.Printf("[I][job] Created logger: %s", j.Logger.outFile.Name())
	}

	if j.Fetcher == nil {
		f := &GithubFetcher{
			BaseDir:   path.Join(fetchBaseDir, s.Owner, s.Repo),
			LogWriter: j.Logger.outFile,
			Source:    s,
		}
		j.Fetcher = f
	}

	if j.Notifier == nil {
		c := NewOAuthGithubClient(j.Config.GithubToken)
		t := j.Config.BaseURL + fmt.Sprintf("/jobs/%s/status/%s", j.ID, sanitizePath(s.Rev))
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
	j.Logger.Printf("[I][job] Fetching started")
	_ = j.Notifier.Notify("pending", "Stage Fetching started")
	err := j.Fetcher.Fetch()
	if err != nil {
		j.Logger.Printf("[E][job] Fetching failed: %v", err)
		_ = j.Notifier.Notify("error", "Stage Fetching failed")
		return err
	}
	j.Logger.Printf("[I][job] Fetching succeeded")

	// Defer Cleanup
	//defer j.Fetcher.Cleanup()

	// Look for manifest
	m, err := FindManifest(j.Fetcher.CheckoutDir())
	if err != nil {
		log.Printf("[E][job] Failed to find valid manifest: %v", err)
		return err
	}

	// Test
	j.Logger.Printf("[I][job] Testing started")
	if err := j.Test(m, j.Fetcher.CheckoutDir()); err != nil {
		j.Logger.Printf("[E][job] Testing failed: %v", err)
		if _, ok := err.(*exec.ExitError); ok {
			_ = j.Notifier.Notify("failure", "Stage Testing failed")
		} else {
			_ = j.Notifier.Notify("error", "Stage Testing failed")
		}
		return err
	}
	j.Logger.Printf("[I][job] Testing succeeded")

	// Done
	_ = j.Notifier.Notify("success", "All stages succeeded")
	return nil
}

// Test runs the tests defined in the manifest.
func (j *Job) Test(m *Manifest, wd string) error {
	dockerWd, err := dockerWd(wd)
	if err != nil {
		return err
	}

	// Set build-specific environment variables
	_ = os.Setenv("WORKSPACE", wd)
	_ = os.Setenv("DOCKER_WORKSPACE", dockerWd)

	env := prepareEnv(m.Environment)

	ctx, cancel := context.WithTimeout(context.Background(), testExecutionTimeout)
	defer cancel()

	for _, line := range m.Test {
		cmd := exec.CommandContext(ctx, line[0], line[1:]...)
		cmd.Dir = wd
		cmd.Env = env
		cmd.Stdout = j.Logger.outFile
		cmd.Stderr = j.Logger.outFile

		j.Logger.Printf("[I][job] Running command: %v (%s)", cmd.Args, cmd.Dir)
		if err := cmd.Run(); err != nil {
			j.Logger.Printf("[I][job] Command failed: %v", err)
			return err
		}
	}

	return nil
}

// LogFilePath assembles a log file path from a job id and revision.
func LogFilePath(jobID, rev string) string {
	saneID := sanitizePath(jobID) // e.g.: scraperwiki/foo
	saneRev := sanitizePath(rev)  // e.g.: refs/origin/master
	return path.Join(logBaseDir, saneID, saneRev, "log.txt")
}

func sanitizePath(path string) string {
	return strings.Replace(strings.Replace(path, "/", "_", -1), ":", "_", -1)
}

func dockerWd(wd string) (string, error) {
	wdOutside, errO := filepath.Abs(os.Getenv("SEAEYE_WORKSPACE"))
	if errO != nil {
		return "", errO
	}

	wdInside, errI := filepath.Abs(fetchBaseDir)
	if errI != nil {
		return "", errI
	}

	if wdOutside == "/" {
		return wdInside, nil
	} else if !strings.HasSuffix(wdOutside, wdInside) {
		return "", fmt.Errorf("non-suffix outside workdirectory")
	}

	dockerWd := path.Join(strings.TrimSuffix(wdOutside, wdInside), wd)
	return dockerWd, nil
}

func prepareEnv(manifestEnv []string) (env []string) {
	envs := append(os.Environ(), manifestEnv...)

	for _, e := range envs {
		if !strings.HasPrefix(e, "SEAEYE_") {
			env = append(env, e)
		}
	}
	return
}
