package seaeye

// Fetcher fetches a repository.
type Fetcher interface {
	Fetch() error
	Cleanup()
	CheckoutDir() string
}
