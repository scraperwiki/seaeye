package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"github.com/scraperwiki/hookbot/pkg/listen"
)

func spawnSubscriber(endpoint string) <-chan []byte {
	msgs, errs := listen.RetryingWatch(endpoint, http.Header{}, nil)
	go errorHandler(errs)

	return msgs
}

func errorHandler(errs <-chan error) {
	for err := range errs {
		log.Println("Warn: Subscription error:", err)
	}
}

func spawnEventHandler(msgs <-chan []byte) chan SubEvent {
	events := make(chan SubEvent)

	go func() {
		for msg := range msgs {
			// HACK: Recursive topic messages are of format '{path}\x00{data}'.
			parts := bytes.Split(msg, []byte{'\x00'})
			if len(parts) == 2 {
				msg = parts[1]
			}

			var event SubEvent
			if err := json.Unmarshal(msg, &event); err != nil {
				log.Printf("Warn: Event error: %v: %v", err, string(msg[:]))
				continue
			}

			events <- event
		}
	}()

	return events
}
