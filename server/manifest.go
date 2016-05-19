package seaeye

// Manifest defines the structure of a .seaeye.yml manifest file.
type Manifest struct {
	ID      string `yaml:",omitempty"`
	Trigger struct {
		Hookbot string `yaml:",omitempty"`
	} `yaml:",omitempty"`
	Checkout struct {
		Github string `yaml:",omitempty"` // SSH key file
	} `yaml:",omitempty"`
	Environment []string   `yaml:",omitempty"`
	Test        [][]string `yaml:",omitempty,flow"`
	Notify      struct {
		Github string `yaml:",omitempty"` // Personal Access token
	} `yaml:",omitempty"`
}

// Parse activates a manifest by setting up the pipeline and starting the
// trigger in the background.
func (m *Manifest) Parse(conf *Config) (*Job, error) {
	t, err := m.parseTrigger()
	if err != nil {
		return nil, err
	}

	n, err := m.parseNotifier()
	if err != nil {
		return nil, err
	}

	// TODO(uwe): Could be stiched together by the caller as it holds conf and
	// manifest.
	j := &Job{
		Config:   conf,
		Manifest: m,
		Notifier: n,
		Trigger:  t,
	}
	return j, nil
}

func (m *Manifest) parseTrigger() (*HookbotTrigger, error) {
	if m.Trigger.Hookbot != "" {
		t := &HookbotTrigger{
			ID:       m.ID,
			Endpoint: m.Trigger.Hookbot,
		}
		return t, nil
	}

	return nil, nil
}

func (m *Manifest) parseNotifier() (*GithubNotifier, error) {
	if m.Notify.Github != "" {
		n := &GithubNotifier{
			Token: m.Notify.Github,
		}
		return n, nil
	}

	return nil, nil
}
