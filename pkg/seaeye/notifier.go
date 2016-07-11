package seaeye

// Notifier notifies an entity given a state and a description.
type Notifier interface {
	Notify(state string, desc string) error
}
