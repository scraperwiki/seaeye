package seaeye

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/scraperwiki/hookbot/pkg/listen"
)

// SubEvent specifies a Hookbot event. Example:
//
// github.com/repo/scraperwiki/pdftables.com/branch/fix-make-it-dev␀{
//   "Branch":"fix-make-it-dev",
//   "Repo":"scraperwiki/pdftables.com",
//   "SHA":"dc1329177775c1c72c0a16f4b2d42d2e3d701c19",
//   "Type":"push",
//   "Who":"djui"
// }
type SubEvent struct {
	Branch string
	Repo   string
	SHA    string
	Type   string
	Who    string
}

// HookbotTrigger specifies a Hookbot subscriber instance.
type HookbotTrigger struct {
	Endpoint string
	Hook     func(e *github.PushEvent) error
	errCh    <-chan error
	finishCh chan struct{}
	msgCh    <-chan []byte
}

// Start starts a retrying Hookbot listener subscribing to a given endpoint.
func (h *HookbotTrigger) Start() error {
	finishCh := make(chan struct{})
	msgCh, errCh := listen.RetryingWatch(h.Endpoint, http.Header{}, finishCh)
	go h.errorHandler(errCh)
	go h.msgHandler(msgCh)
	return nil
}

// Stop unsubscribes and stops a retrying Hookbot listener.
func (h *HookbotTrigger) Stop() error {
	h.finishCh <- struct{}{}
	close(h.finishCh)
	return nil
}

func (h *HookbotTrigger) errorHandler(errCh <-chan error) {
	for err := range errCh {
		log.Printf("[W][trigger_hookbot] Subscription error for %s: %v", h.Endpoint, err)
	}
}

func (h *HookbotTrigger) msgHandler(msgCh <-chan []byte) {
	for msg := range msgCh {
		// Recursive topic messages are of format '{path}␀{data}'.
		parts := bytes.Split(msg, []byte{'\x00'})
		if len(parts) == 2 {
			msg = parts[1]
		}

		var event SubEvent
		if err := json.Unmarshal(msg, &event); err != nil {
			log.Printf("[W][trigger_hookbot] Event [E]%v: %s", err, string(msg[:]))
			continue
		}

		log.Printf("[D][trigger_hookbot] Event: %s %s", event.Repo, event.SHA)

		if event.Type != "push" {
			continue
		}

		// DeletedBranchHash defines the magic git commit hash for a deleted branch.
		const DeletedBranchHash = "0000000000000000000000000000000000000000"
		if event.SHA == DeletedBranchHash {
			continue
		}

		ref := fmt.Sprintf("refs/heads/%s", event.Branch)
		sshURL := fmt.Sprintf("git@github.com:%s.git", event.Repo)

		// Convert Hookbot subscription event back to Github WebHook Push event.
		e := github.PushEvent{
			After: &event.SHA,
			Pusher: &github.User{
				Name: &event.Who,
			},
			Repo: &github.PushEventRepository{
				FullName: &event.Repo,
				URL:      &sshURL,
			},
			Ref: &ref,
		}

		if h.Hook == nil {
			log.Printf("[W][trigger_hookbot] No hook defined for: %v", e)
			continue
		}

		// Execute hooks sequential for now, as parallel will likely cause
		// resource conflicts and/or race-conditions.
		if err := h.Hook(&e); err != nil {
			log.Printf("[E][trigger_hookbot] Hook failed: %v %v", e, err)
		}
	}
}
