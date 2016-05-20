package seaeye

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/go-github/github"
)

// App specifies an application state and lifecycle.
type App struct {
	Builds    *BuildQueue
	Config    *Config
	Hookbot   *HookbotTrigger
	WebServer *Server
	startTime time.Time
}

// BuildQueue specifies a sequential queue of pending builds.
type BuildQueue struct {
	BuildCh chan *Build
	doneCh  chan struct{}
}

// Build specifies a specific build for a job given a github push event as
// parameter.
type Build struct {
	Job    *Job
	Source *Source
}

// Stats contains statistics about the application.
type Stats map[string]interface{}

// Start starts the server: web > build > hookbot > signals.
func (a *App) Start() error {
	log.Println("[I][app] Starting")
	if a.startTime.IsZero() {
		a.startTime = time.Now()
	}

	if a.WebServer == nil {
		log.Println("[I][app] Creating web server")
		a.WebServer = NewWebServer(a.Config, a.Builds.BuildCh, a.stats)
	}
	log.Printf("[I][app] Starting web server %s", a.WebServer.Addr)
	if err := a.WebServer.Start(); err != nil {
		log.Printf("[E][app] Failed to start web server: %v", err)
		return err
	}

	if a.Builds == nil {
		log.Println("[I][app] Creating build queue")
		a.Builds = &BuildQueue{
			BuildCh: make(chan *Build, 50),
			doneCh:  make(chan struct{}),
		}
	}
	log.Printf("[I][app] Waiting for builds: %d", cap(a.Builds.BuildCh))
	go waitForBuilds(a.Builds)

	if a.Hookbot == nil {
		log.Println("[I][app] Creating hookbot subscriber")
		a.Hookbot = &HookbotTrigger{
			Endpoint: a.Config.HookbotEndpoint,
			Hook: func(event *github.PushEvent) error {
				g := &GithubTrigger{}
				url := a.Config.HostPort + "/webhook"
				return g.Post(url, event)
			},
		}
	}
	log.Printf("[I][app] Starting hookbot subscriber: %s", a.Config.HookbotEndpoint)
	a.Hookbot.Start()

	log.Println("[I][app] Waiting for signals")
	a.waitForSignals()
	log.Println("[I][app] Started")
	return nil
}

// Stop shuts down the server: hookbot > build > web.
func (a *App) Stop() error {
	log.Println("[I][app] Stopping")

	if a.Hookbot != nil {
		log.Println("[I][app] Stopping hookbot subscriber")
		a.Hookbot.Stop()
	}

	if a.Builds != nil {
		log.Println("[I][app] Closing build queue")
		a.Builds.doneCh <- struct{}{}
	}

	if a.WebServer != nil {
		log.Println("[I][app] Stopping web server")
		if err := a.WebServer.Stop(); err != nil {
			log.Printf("[E][app] Failed to stop web server: %v", err)
			return err
		}
	}

	log.Println("[I][app] Stopped")
	return nil
}

// waitForBuilds sequentially executes builds by applying a push webhook to a
// job.
func waitForBuilds(builds *BuildQueue) {
	for {
		select {
		case b, _ := <-builds.BuildCh:
			if err := b.Job.Execute(b.Source); err != nil {
				log.Printf("[E][app] Build failed: %v", err)
			}
		case <-builds.doneCh:
			return
		}
	}
}

func (a *App) waitForSignals() {
	sigc := make(chan os.Signal, 6)
	signal.Notify(sigc,
		syscall.SIGUSR1, // syscall.SIGINFO,
		syscall.SIGHUP, syscall.SIGUSR2,
		syscall.SIGINT, syscall.SIGTERM,
	)
	for sig := range sigc {
		switch sig {
		case // syscall.SIGINFO, // Ctrl-t // BSD only
			syscall.SIGUSR1:
			a.printStats()
		case syscall.SIGHUP,
			syscall.SIGUSR2:
			a.reload()
		case syscall.SIGINT, // Ctrl-c
			syscall.SIGTERM:
			signal.Stop(sigc)
			return
		}
	}
}

func (a *App) reload() {
	log.Println("[I][app] Reloading")
	// TODO(uwe): Do reloading
}

func (a *App) printStats() {
	stats := a.stats()
	for k, v := range stats {
		log.Printf("[I][app] Stats %s: %v", k, v)
	}
}

func (a *App) stats() Stats {
	return map[string]interface{}{
		"/app/build_queue/count":        len(a.Builds.BuildCh),
		"/app/start_time":               a.startTime,
		"/app/uptime":                   time.Now().Sub(a.startTime),
		"/app/version":                  a.Config.Version,
		"/webserver/active_connections": a.WebServer.ConnActive,
	}
}