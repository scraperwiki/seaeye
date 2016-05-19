package seaeye

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// App specifies an application state and lifecycle.
type App struct {
	conf      *Config
	builds    *BuildQueue
	server    *Server
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
	Job       *Job
	Parameter *GithubPushEvent
}

// New instantiates a new application.
func New() *App {
	return &App{conf: NewConfig()}
}

// Start is the main entrypoint for the server.
func (a *App) Start() error {
	if a.startTime.IsZero() {
		a.startTime = time.Now()
	}
	log.Println("Info: [app] Starting")

	log.Println("Info: [app] Loading manifest store")
	manifestStore := NewManifestStore()

	log.Println("Info: [app] Creating build queue")
	a.builds = &BuildQueue{
		BuildCh: make(chan *Build, 10),
		doneCh:  make(chan struct{}),
	}
	go waitForBuilds(a.builds)

	a.server = NewServer(a.conf, manifestStore, a.builds.BuildCh)
	log.Printf("Info: [app] Starting web server %s", a.server.Addr)
	if err := a.server.Start(); err != nil {
		log.Printf("Error: [app] Can't start web server: %v", err)
		return err
	}

	log.Println("Info: [app] Started")
	a.waitForSignals()
	return nil
}

// Stop shuts down the server.
func (a *App) Stop() error {
	log.Println("Info: [app] Stopping")

	log.Println("Info: [app] Stopping web server")
	if err := a.server.Stop(); err != nil {
		log.Printf("Error: [app] Failed to stop web server: %v", err)
		return err
	}

	log.Println("Info: [app] Closing build queue")
	a.builds.doneCh <- struct{}{}
	log.Println("Info: [app] Closed build queue")

	log.Println("Info: [app] Stopped")
	return nil
}

// waitForBuilds sequentially executes builds by applying a push webhook to a
// job.
func waitForBuilds(builds *BuildQueue) {
	for {
		select {
		case b, _ := <-builds.BuildCh:
			if err := b.Job.Execute(b.Parameter); err != nil {
				log.Println("Warn: [app] Build failed: ")
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
	log.Println("Info: [app] Reloading")
	// TODO(uwe): Do reloading
}

func (a *App) printStats() {
	log.Printf("Info: [app] Stats /app/start_time: %v", a.startTime)
	log.Printf("Info: [app] Stats /app/uptime: %v", time.Now().Sub(a.startTime))
	log.Printf("Info: [app] Stats /app/build_queue/count: %v", len(a.builds.BuildCh))
	log.Printf("Info: [app] Stats /server/active_connections: %v", a.server.ConnActive)
}
