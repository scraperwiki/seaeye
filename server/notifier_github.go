package seaeye

import (
	"log"

	"github.com/google/go-github/github"
)

// GithubNotifier updates a repository commit state on Github.
type GithubNotifier struct {
	Client    *OAuthGithubClient
	Source    *Source
	TargetURL string
}

// Notify notifies a Github repository about status updates. State is required,
// desc is optional.
func (g *GithubNotifier) Notify(state, desc string) error {
	context := "ci" // "seaeye"
	s := &github.RepoStatus{
		Context:     &context,
		Description: &desc,
		State:       &state,
		TargetURL:   &g.TargetURL,
	}

	log.Printf("[I][notifier_github] Notifying Github: %s - %s", *s.State, *s.Description)
	_, resp, err := g.Client.Repositories.CreateStatus(g.Source.Owner, g.Source.Repo, g.Source.Rev, s)
	if err != nil {
		log.Printf("[E][notifier_github] Failed to notify Github: %v", err)
		return err
	}
	defer resp.Body.Close()

	return nil
}
