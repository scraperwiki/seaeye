package seaeye

import (
	"log"
	"net/url"
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

// Stats contains statistics about the application.
type Stats map[string]interface{}

// Start starts the server: build > web > hookbot > signals.
func (a *App) Start() error {
	log.Println("[I][app] Starting")
	if a.startTime.IsZero() {
		a.startTime = time.Now()
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

	if a.WebServer == nil {
		log.Println("[I][app] Creating web server")
		a.WebServer = NewWebServer(a.Config, a.Builds.BuildCh, a.stats)
	}
	log.Printf("[I][app] Starting web server %s", a.WebServer.Addr)
	if err := a.WebServer.Start(); err != nil {
		log.Printf("[E][app] Failed to start web server: %v", err)
		return err
	}

	if a.Hookbot == nil {
		log.Println("[I][app] Creating hookbot subscriber")
		a.Hookbot = &HookbotTrigger{
			Endpoint: a.Config.HookbotEndpoint,
			Hook: func(event *github.PushEvent) error {
				g := &GithubTrigger{}
				url := &url.URL{Scheme: "http", Host: a.Config.HostPort, Path: "webhook"}
				return g.Post(url.String(), event)
			},
		}
	}
	log.Printf("[I][app] Starting hookbot subscriber: %s", a.Config.HookbotEndpoint)
	if err := a.Hookbot.Start(); err != nil {
		log.Printf("[E][app] Failed to start hookbot subscriber: %v", err)
		return err
	}

	log.Println("[I][app] Started")
	return nil
}

// Stop shuts down the server: hookbot > web > build.
func (a *App) Stop() error {
	log.Println("[I][app] Stopping")

	if a.Hookbot != nil {
		log.Println("[I][app] Stopping hookbot subscriber")
		a.Hookbot.Stop()
	}

	if a.WebServer != nil {
		log.Println("[I][app] Stopping web server")
		if err := a.WebServer.Stop(); err != nil {
			log.Printf("[E][app] Failed to stop web server: %v", err)
			return err
		}
	}

	if a.Builds != nil {
		log.Println("[I][app] Closing build queue")
		a.Builds.doneCh <- struct{}{}
	}

	log.Println("[I][app] Stopped")
	return nil
}

// WaitForSignals listens looping for syscall signals until SIGINT or SIGTERM is
// provided.
func (a *App) WaitForSignals() {
	log.Println("[I][app] Waiting for signals")
	sigc := make(chan os.Signal, 6)
	signal.Notify(sigc,
		syscall.SIGUSR1, // syscall.SIGINFO, // BSD only
		syscall.SIGHUP, syscall.SIGUSR2,
		syscall.SIGINT, syscall.SIGTERM,
	)
	for sig := range sigc {
		switch sig {
		case // syscall.SIGINFO, // Ctrl-t
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
