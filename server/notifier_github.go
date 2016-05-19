package seaeye

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
)

// GithubAPILink is the Github API link that allows adding status updated.
//
// NOTE: Having the access_token in the url makes is better to debug but a
// security issue for the logs.
const GithubAPILink string = "https://api.github.com/repos/%s/statuses/%s?access_token=%s"

// GithubNotifier updates a repository commit state on Github.
type GithubNotifier struct {
	Token     string
	mutex     sync.RWMutex
	apiURL    string
	repo      string
	rev       string
	targetURL string
}

// CommitStatus specifies the Github API reponse for commit statuses.
type CommitStatus struct {
	Context     string `json:"context,omitempty"`
	Description string `json:"description,omitempty"`
	State       string `json:"state"`
	TargetURL   string `json:"target_url,omitempty"`
}

// SetContext sets the template values for all upcoming notifications. Repo and
// rev are required, targetURL is optional.
func (g *GithubNotifier) SetContext(repo, rev, targetURL string) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.apiURL = fmt.Sprintf(GithubAPILink, repo, rev, g.Token)
	g.repo = repo
	g.rev = rev
	g.targetURL = targetURL
}

// Notify notifies a Github repository about status updates. State is required,
// desc is optional.
func (g *GithubNotifier) Notify(state, desc string) error {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	if g.apiURL == "" {
		return fmt.Errorf("context not set")
	}

	status := &CommitStatus{
		Context:     "ci", // "seaeye"
		Description: desc,
		State:       state,
		TargetURL:   g.targetURL,
	}

	log.Printf("Info: [notifier_github] Notifying Github: %s - %s (%s)",
		status.State, status.Description, status.TargetURL)

	if err := g.post(g.apiURL, status); err != nil {
		log.Printf("Error: [notifier_github]  %v", err)
		return err
	}

	return nil
}

// For authentication consult https://developer.github.com/v3/#authentication.
func (g *GithubNotifier) post(url string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("posting request failed: %s: %v", resp.Status, string(body))
	}

	return nil
}
