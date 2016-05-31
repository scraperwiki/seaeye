package seaeye

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Source represents a specific snapshot of a Github remote repository.
type Source struct {
	Owner, Repo, Rev, URL string
}

// OAuthGithubClient is a thin wrapper around the google/go-github client with
// implicit OAuth setup and a bit of extending functionality.
type OAuthGithubClient struct {
	*github.Client
}

// NewOAuthGithubClient instanciates a new OAuth Github client given a personal
// access token.
func NewOAuthGithubClient(token string) *OAuthGithubClient {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	return &OAuthGithubClient{Client: github.NewClient(tc)}
}

// PushEventFromRequest parses the body of a POST request and returns a
// (minimal) Github API v3 push event.
func PushEventFromRequest(req *http.Request) (*github.PushEvent, error) {
	contentTypeHeader := req.Header.Get("Content-Type")
	contentType, _, _ := mime.ParseMediaType(contentTypeHeader)
	if contentType != "application/json" && contentType != "application/x-www-form-urlencoded" {
		return nil, fmt.Errorf("unsupported content-type: %s", contentType)
	}
	eventTypeHeader := req.Header.Get("X-Github-Event")
	if eventTypeHeader != "push" {
		return nil, fmt.Errorf("unsupported event-type: %s", eventTypeHeader)
	}

	var e github.PushEvent
	if err := json.NewDecoder(req.Body).Decode(&e); err != nil {
		return nil, fmt.Errorf("failed to parse event: %v", err)
	}

	return &e, nil
}
