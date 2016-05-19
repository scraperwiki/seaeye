package seaeye

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

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

// Listeners holds a synchronizable map of id to listener's finish channel.
type Listeners struct {
	m     map[string]chan struct{}
	mutex sync.RWMutex
}

var listeners = &Listeners{m: make(map[string]chan struct{})}

// HookbotTrigger specifies a Hookbot subscriber instance.
type HookbotTrigger struct {
	ID       string
	Endpoint string
	Hook     func(e *GithubPushEvent) error
}

// Start starts a retrying Hookbot listener subscribing to a given endpoint.
func (h *HookbotTrigger) Start() error {
	finishCh := make(chan struct{})
	msgCh, errCh := listen.RetryingWatch(h.Endpoint, http.Header{}, finishCh)
	go h.errorHandler(errCh)
	go h.msgHandler(msgCh)

	listeners.mutex.Lock()
	listeners.m[h.ID] = finishCh
	listeners.mutex.Unlock()

	return nil
}

// Stop unsubscribes and stops a retrying Hookbot listener.
func (h *HookbotTrigger) Stop() error {
	// TODO(uwe): Not really unique criteria. Multiple manifest could have the
	// same hookbot trigger endpoint.
	t, ok := listeners.m[h.ID]
	if !ok {
		return fmt.Errorf("no trigger found")
	}

	listeners.mutex.Lock()
	close(t)
	delete(listeners.m, h.ID)
	listeners.mutex.Unlock()

	return nil
}

func (h *HookbotTrigger) errorHandler(errCh <-chan error) {
	for err := range errCh {
		log.Printf("Warn: [trigger_hookbot] Subscription error for %s: %v", h.Endpoint, err)
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
			log.Printf("Warn: [trigger_hookbot] Event error: %v: %s", err, string(msg[:]))
			continue
		}

		log.Printf("Debug: [trigger_hookbot] Event: %s %s", event.Repo, event.SHA)

		if event.Type != "push" {
			continue
		}

		// DeletedBranchHash defines the magic git commit hash for a deleted branch.
		const DeletedBranchHash = "0000000000000000000000000000000000000000"
		if event.SHA == DeletedBranchHash {
			continue
		}

		if h.Hook == nil {
			log.Println("Warn: [trigger_hookbot] No hook specified. Dropping event")
			continue
		}

		// Convert Hookbot subscription event back to Github WebHook Push event.
		e := &GithubPushEvent{}
		e.After = event.SHA
		e.Pusher.Name = event.Who
		e.Repository.FullName = event.Repo
		e.Repository.SSHURL = fmt.Sprintf("git@github.com:%s.git", event.Repo)
		e.Ref = fmt.Sprintf("refs/heads/%s", event.Branch)

		// Execute hooks sequential for now, as parallel will likely cause
		// resource conflicts and/or race-conditions.
		if err := h.Hook(e); err != nil {
			log.Printf("Error: [trigger_hookbot] Hook failed: %v %v", e, err)
		}
	}
}
