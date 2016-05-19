package seaeye

import "sync"

// ManifestStore holds a set of manifest.
type ManifestStore struct {
	data map[string]*Manifest
	sync.RWMutex
}

// NewManifestStore instantiates a new manifest store.
func NewManifestStore() *ManifestStore {
	return &ManifestStore{data: make(map[string]*Manifest)}
}

// Get gets a manifest from the manifest store.
func (m *ManifestStore) Get(k string) (*Manifest, bool) {
	m.RLock()
	defer m.RUnlock()
	manifest, ok := m.data[k]
	return manifest, ok
}

// Put puts or deletes a manifest into/from the manifest store.
func (m *ManifestStore) Put(k string, v *Manifest) {
	m.Lock()
	if v == nil {
		delete(m.data, k)
	} else {
		m.data[k] = v
	}
	m.Unlock()
}
