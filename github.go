package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

const GH_LINK string = "https://api.github.com/repos/%s/statuses/%s"

type Notifier func(string, interface{}) error

func spawnGithubNotifier(statuses <-chan CommitStatus, notify Notifier, urlPrefix string) {
	go func() {
		for status := range statuses {
			status.Context = "ci"
			status.TargetUrl = fmt.Sprint(urlPrefix, status.TargetUrl)
			url := fmt.Sprintf(GH_LINK, status.Repo, status.Rev)
			log.Println("Info: Notify Github:", url, status)
			if err := notify(url, status); err != nil {
				log.Println("Error:", err)
			}
		}
	}()
}

func githubRestPOST(user string, token string) Notifier {
	return func(url string, payload interface{}) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("POST", url, bytes.NewReader(data))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		// https://developer.github.com/v3/#Authentication
		//req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
		req.SetBasicAuth(user, token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			body, _ := ioutil.ReadAll(resp.Body)
			return fmt.Errorf("%s: %v", resp.Status, body)
		}

		return nil
	}
}
