package seaeye

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/scraperwiki/git-prep-directory"
)

// GithubFetcher manages cloned Github repositories locally.
type GithubFetcher struct {
	BaseDir   string
	LogWriter io.Writer
	Source    *Source
	buildDir  *git.BuildDirectory
}

// Fetch clones a Github repositry and checks out a given revision.
func (g *GithubFetcher) Fetch() error {
	log.Printf("[I][fetcher_github] Running git-prep-directory: %s %s %s",
		g.BaseDir, g.Source.URL, g.Source.Rev)
	buildDir, err := git.PrepBuildDirectory(g.BaseDir, g.Source.URL, g.Source.Rev, 10*time.Minute, g.LogWriter)
	if err != nil {
		log.Printf("[E][fetcher_github] Fetch failed: %v", err)
		return fmt.Errorf("fetch for %s %s failed: %v", g.Source.URL, g.Source.Rev, err)
	}

	g.buildDir = buildDir
	log.Printf("[I][fetcher_github] Fetch succeeded: %s", g.buildDir.Dir)
	return nil
}

// Cleanup removes all checked out files.
func (g *GithubFetcher) Cleanup() {
	if g.buildDir == nil {
		return
	}

	log.Printf("[I][fetcher_github] Starting cleanup")
	g.buildDir.Cleanup()
	log.Printf("[I][fetcher_github] Cleanup finished")
}

// CheckoutDir returns the directory of the checked out files.
func (g *GithubFetcher) CheckoutDir() string {
	if g.buildDir == nil {
		return ""
	}
	return g.buildDir.Dir
}
