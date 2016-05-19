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
	"mime"
	"net/http"
)

// GithubPushEvent describes a minimal schema of a Github API v3 push event.
type GithubPushEvent struct {
	After      string `json:",omitempty"`
	Ref        string `json:",omitempty"`
	Repository struct {
		FullName string `json:"full_name,omitempty"`
		SSHURL   string `json:"ssh_url,omitempty"`
	} `json:",omitempty"`
	Pusher struct {
		Name string `json:",omitempty"`
	} `json:",omitempty"`
}

// GithubTrigger can parse and send (minimal) Github API v3 push events.
type GithubTrigger struct{}

// PushEventFromRequest parses the body of a POST request and returns a
// (minimal) Github API v3 push event.
func (g *GithubTrigger) PushEventFromRequest(req *http.Request) (*GithubPushEvent, error) {
	contentTypeHeader := req.Header.Get("Content-Type")
	contentType, _, _ := mime.ParseMediaType(contentTypeHeader)
	if contentType != "application/json" && contentType != "application/x-www-form-urlencoded" {
		return nil, fmt.Errorf("unsupported content-type: %s", contentType)
	}
	eventTypeHeader := req.Header.Get("X-Github-Event")
	if eventTypeHeader != "push" {
		return nil, fmt.Errorf("unsupported event-type: %s", eventTypeHeader)
	}

	var e GithubPushEvent
	if err := json.NewDecoder(req.Body).Decode(&e); err != nil {
		return nil, fmt.Errorf("failed to parse event: %v", err)
	}

	return &e, nil
}

// Post sends a (minimal) Github API v3 push event to a given URL.
func (g *GithubTrigger) Post(url string, e *GithubPushEvent) error {
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
