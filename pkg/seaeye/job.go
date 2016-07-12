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
	Config   *Config     // ...to prefix targetURL with BaseURL.
	Fetcher  Fetcher     // ...to clone git repo.
	ID       string      // ...to identify for logs.
	Logger   *FileLogger // ...to accessed persistent and durable logs via REST endpoint.
	Manifest *Manifest   // ...to make testing easier.
	Notifier Notifier    // ...to update commmit statuses.
}

type step struct {
	name         string
	instructions [][]string
	relevant     bool
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

	if j.Config.NoNotify {
		j.Notifier = &DiscardNotifier{}
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
	//_ = j.Notifier.Notify("pending", "Starting...")

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
	//_ = j.Notifier.Notify("pending", "Stage Fetching started")
	if err := j.Fetcher.Fetch(); err != nil {
		j.Logger.Printf("[E][job] %s Fetching failed: %v", j.ID, err)
		_ = j.Notifier.Notify("error", "Stage Fetching failed")
		return err
	}
	j.Logger.Printf("[I][job] %s Fetching succeeded", j.ID)

	// Defer Cleanup
	//defer j.Fetcher.Cleanup()

	// Prepare
	j.Logger.Printf("[I][job] %s Preparing started", j.ID)
	//_ = j.Notifier.Notify("pending", "Stage Preparing started")
	wd, err := filepath.Abs(j.Fetcher.CheckoutDir())
	if err != nil {
		j.Logger.Printf("[E][job] %s Preparation failed: %v", j.ID, err)
		_ = j.Notifier.Notify("error", "Stage Preparing failed")
		return err
	}

	if j.Manifest == nil {
		j.Logger.Printf("[I][job] %s Looking for manifest: %v", j.ID, wd)
		m, err := FindManifest(wd)
		if err != nil {
			j.Logger.Printf("[E][job] %s Failed to find valid manifest: %v", j.ID, err)
			// Report no manifest found as success to Github as we can't
			// distinguish if that was intended or not.
			//_ = j.Notifier.Notify("success", "No manifest found")
			return err
		}
		j.Manifest = m
	}

	env := j.prepareEnv(wd)

	steps := []*step{
		&step{name: "Pre", instructions: j.Manifest.Pre},
		&step{name: "Test", instructions: j.Manifest.Test, relevant: true},
		&step{name: "Post", instructions: j.Manifest.Post},
	}

	var firstRelevantErr error

	for _, step := range steps {
		j.Logger.Printf("[I][job] %s %s started", j.ID, step.name)
		_ = j.Notifier.Notify("pending", fmt.Sprintf("Stage %s started", step.name))
		err := j.ExecuteStep(step.instructions, wd, env)
		j.Logger.Printf("[I][job] %s %s finished", j.ID, step.name)

		if err != nil {
			j.Logger.Printf("[E][job] %s %s failed: %v", j.ID, step.name, err)
			if step.relevant {
				if firstRelevantErr == nil {
					firstRelevantErr = err
				}
				if _, ok := err.(*exec.ExitError); ok {
					_ = j.Notifier.Notify("failure", fmt.Sprintf("Stage %s failed", step.name))
				} else {
					_ = j.Notifier.Notify("error", fmt.Sprintf("Stage %s failed", step.name))
				}
			}
		} else {
			j.Logger.Printf("[I][job] %s %s succeeded", step.name, j.ID)
		}
	}

	// Done
	if firstRelevantErr == nil {
		_ = j.Notifier.Notify("success", "All stages succeeded")
	}
	return firstRelevantErr
}

// ExecuteStep executes instructions defined in a manifest step.
func (j *Job) ExecuteStep(instructions [][]string, wd string, env []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), j.Config.ExecTimeout)
	defer cancel()

	for _, line := range instructions {
		cmd := exec.CommandContext(ctx, line[0], line[1:]...)
		cmd.Dir = wd
		cmd.Env = env
		cmd.Stdout = j.Logger.outFile
		cmd.Stderr = j.Logger.outFile

		j.Logger.Printf("[I][job] %s Running command: %v (%s)", j.ID, cmd.Args, cmd.Dir)
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				j.Logger.Printf("[I][job] %s Command failed: %v", j.ID, exitErr)
			} else {
				j.Logger.Printf("[I][job] %s Command error: %v", j.ID, err)
			}
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
