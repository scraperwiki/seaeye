package seaeye

// DiscardNotifier discard the notification to be sent.
type DiscardNotifier struct{}

// Notify discards the notification.
func (n *DiscardNotifier) Notify(state, desc string) error {
	return nil
}
