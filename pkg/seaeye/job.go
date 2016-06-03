package seaeye

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/scraperwiki/seaeye/pkg/exec"
	"golang.org/x/net/context"
)

// Job is responsible for an describes all necessary modules to execute a job.
type Job struct {
	Config   *Config         // ...to prefix targetURL with BaseURL.
	Fetcher  *GithubFetcher  // ...to clone git repo.
	ID       string          // ...to identify for logs.
	Logger   *FileLogger     // ...to accessed persistent and durable logs via REST endpoint.
	Manifest *Manifest       // ...to make testing easier.
	Notifier *GithubNotifier // ...to update commmit statuses.
}

// Execute executes a given task: 1. Setup, 2. Run (2a. Fetch, 2b. Test).
func (j *Job) Execute(s *Source) error {
	if err := j.setup(s); err != nil {
		return err
	}
	defer j.Logger.outFile.Close()

	j.Logger.Printf("[I][job] %s Running", j.ID)
	if err := j.run(); err != nil {
		j.Logger.Printf("[E][job] %s Run failed: %v", j.ID, err)
		return err
	}

	return nil
}

// Setup ensures that all relevant job parts are configured and instatiated.
func (j *Job) setup(s *Source) error {
	if j.ID == "" {
		j.ID = escapePath(path.Join(s.Owner, s.Repo))
	}

	if j.Logger == nil {
		logFilePath, err := j.Config.LogFilePath(j.ID, s.Rev)
		if err != nil {
			return err
		}
		logger, err := NewFileLogger(logFilePath, log.Prefix(), log.LstdFlags)
		if err != nil {
			return err
		}
		j.Logger = logger
		j.Logger.Printf("[I][job] %s Created logger: %s", j.ID, j.Logger.outFile.Name())
	}

	if j.Fetcher == nil {
		f := &GithubFetcher{
			BaseDir:   path.Join(j.Config.FetchBaseDir, s.Owner, s.Repo),
			LogWriter: j.Logger.outFile,
			Source:    s,
		}
		j.Fetcher = f
	}

	if j.Notifier == nil {
		c := NewOAuthGithubClient(j.Config.GithubToken)
		t := j.Config.BaseURL + fmt.Sprintf("/jobs/%s/status/%s", j.ID, escapePath(s.Rev))
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
	j.Logger.Printf("[I][job] %s Fetching started", j.ID)
	_ = j.Notifier.Notify("pending", "Stage Fetching started")
	if err := j.Fetcher.Fetch(); err != nil {
		j.Logger.Printf("[E][job] %s Fetching failed: %v", j.ID, err)
		_ = j.Notifier.Notify("error", "Stage Fetching failed")
		return err
	}
	j.Logger.Printf("[I][job] %s Fetching succeeded", j.ID)

	// Defer Cleanup
	//defer j.Fetcher.Cleanup()

	if j.Manifest == nil {
		// Look for manifest
		m, err := FindManifest(j.Fetcher.CheckoutDir())
		if err != nil {
			j.Logger.Printf("[E][job] %s Failed to find valid manifest: %v", j.ID, err)
			return err
		}
		j.Manifest = m
	}

	// Test
	j.Logger.Printf("[I][job] %s Testing started", j.ID)
	wd, err := filepath.Abs(j.Fetcher.CheckoutDir())
	if err != nil {
		j.Logger.Printf("[E][job] %s Testing preparation failed: %v", j.ID, err)
		return err
	}
	env := j.prepareEnv(wd)
	if err := j.Test(wd, env); err != nil {
		j.Logger.Printf("[E][job] %s Testing failed: %v", j.ID, err)
		if _, ok := err.(*exec.ExitError); ok {
			_ = j.Notifier.Notify("failure", "Stage Testing failed")
		} else {
			_ = j.Notifier.Notify("error", "Stage Testing failed")
		}
		return err
	}
	j.Logger.Printf("[I][job] %s Testing succeeded", j.ID)

	// Done
	_ = j.Notifier.Notify("success", "All stages succeeded")
	return nil
}

// Test runs the tests defined in the manifest.
func (j *Job) Test(wd string, env []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), j.Config.ExecTimeout)
	defer cancel()

	instructions := j.Manifest.Test
	for _, line := range instructions {
		cmd := exec.CommandContext(ctx, line[0], line[1:]...)
		cmd.Dir = wd
		cmd.Env = env
		cmd.Stdout = j.Logger.outFile
		cmd.Stderr = j.Logger.outFile

		j.Logger.Printf("[I][job] %s Running command: %v (%s)", j.ID, cmd.Args, cmd.Dir)
		if err := cmd.Run(); err != nil {
			j.Logger.Printf("[I][job] %s Command failed: %v", j.ID, err)
			return err
		}
		j.Logger.Printf("[I][job] %s Command succeeded.", j.ID)
	}

	return nil
}

func (j *Job) prepareEnv(wd string) (env []string) {
	// Append only environment variables that are not meant for internal use
	// only or belong to this job.
	jobEnv := envVarCompliant(strings.ToUpper(j.ID))
	jobEnvPrefix := fmt.Sprintf("%s%s_", internalEnvPrefix, jobEnv)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, jobEnvPrefix) {
			env = append(env, strings.TrimPrefix(e, jobEnvPrefix))
		} else if !strings.HasPrefix(e, internalEnvPrefix) {
			env = append(env, e)
		}
	}

	// Append build-specific environment variables
	env = append(env, fmt.Sprintf(`WORKSPACE=%s`, wd))
	env = append(env, fmt.Sprintf(`DOCKER_WORKSPACE=%s`, path.Join(j.Config.DockerHostVolumeBaseDir, wd)))

	// Append manifest environment variables
	env = append(env, j.Manifest.Environment...)

	return env
}

var posixEnvVarPattern = regexp.MustCompile(`[A-Z_]+[0-9A-Z_]+`)
var posixEnvVarBlacklistPattern = regexp.MustCompile(`[^0-9A-Z_]`)

// envVarCompliant takes an environment variable name and alters it become
// compliant with the IEEE Std 1003.1-2008 / IEEE POSIX P1003.2/ISO 9945.2
// standard.
//
// See http://www.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_10_02
func envVarCompliant(name string) string {
	newName := posixEnvVarBlacklistPattern.ReplaceAllLiteralString(name, "")
	return posixEnvVarPattern.FindString(newName)
}
