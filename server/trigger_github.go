// See https://developer.github.com/webhooks/ for event types. The event of
// interest (and default event) is `push` and described under
// https://developer.github.com/v3/activity/events/types/#pushevent
// documentation.

package seaeye

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/google/go-github/github"
)

// GithubTrigger can parse and send (minimal) Github API v3 push events.
type GithubTrigger struct{}

// Post sends a (minimal) Github API v3 push event to a given URL.
func (g *GithubTrigger) Post(url string, e *github.PushEvent) error {
	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("failed to marshal github push event: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Seaeye-Hookbot-Proxy")
	req.Header.Set("X-GitHub-Event", "push")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %v", err)
		}
		return fmt.Errorf("failed to send request: %s: %s", resp.Status, string(b))
	}
	return nil
}
