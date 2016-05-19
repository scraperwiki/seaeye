package seaeye

// Manifest defines the structure of a .seaeye.yml manifest file.
type Manifest struct {
	ID          string     `yaml:",omitempty"`
	Environment []string   `yaml:",omitempty"`
	Test        [][]string `yaml:",omitempty,flow"`
}

// Validate checks the validity of a manifest.
func (m *Manifest) Validate() error {
	// TODO(uwe): Eventual check manifest first
	return nil
}
