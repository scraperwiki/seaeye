package seaeye

import "log"

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

// waitForBuilds sequentially executes builds given a build source as parameter.
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
