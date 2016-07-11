package seaeye

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"gopkg.in/yaml.v2"
)

// ErrManifestNotFound defines that no manifest file could be found.
var ErrManifestNotFound = errors.New("no manifest file found")

// Manifest defines the structure of a .seaeye.yml manifest file.
type Manifest struct {
	Environment []string   `yaml:",omitempty"`
	Pre         [][]string `yaml:",omitempty,flow"`
	Test        [][]string `yaml:",omitempty,flow"`
	Post        [][]string `yaml:",omitempty,flow"`
}

// Validate checks the validity of a manifest.
func (m *Manifest) Validate() error {
	// TODO(uwe): Eventual check manifest first
	return nil
}

// FindManifest looks for a .seaeye.yml file in a given directory and tries to
// parse this file as manifest.
func FindManifest(wd string) (*Manifest, error) {
	var b []byte
	var err error

	manifestFilenames := []string{".seaeye.yml", ".seaeye.yaml"}
	for _, manifestFilename := range manifestFilenames {
		manifestPath := path.Join(wd, manifestFilename)
		b, err = ioutil.ReadFile(manifestPath)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("failed to read manifest file %s: %v", manifestPath, err)
		} else {
			break
		}
	}
	if err != nil || len(b) == 0 {
		return nil, ErrManifestNotFound
	}

	m := &Manifest{}
	if err := yaml.Unmarshal(b, m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %v", err)
	}

	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate manifest: %v", err)
	}

	return m, nil
}
