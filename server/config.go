package seaeye

import (
	"log"
	"os"
)

const (
	configDefaultHostPort = ":19515" // "SEAE"(YE)
	configDefaultBaseURL  = "http://localhost:19515"
)

// Config specifies the configuration to run the seaeye application.
type Config struct {
	// BaseURL holds Seaeye's link scheme, authority, and port.
	BaseURL string
	// GithubToken holds a Personal Access Token for Github to authenticate
	// commit status updates via Github API.
	GithubToken string
	// HookbotEndpoint holds a hookbot subscription URL.
	HookbotEndpoint string
	// HostPort holds Seaeye's server host and port.
	HostPort string
	// Seaeye version
	Version string
}

// NewConfig creates a new configuration with a mix of default values and
// provided environment variables.
func NewConfig() *Config {
	log.Println("Info: [config] Loading configuration")

	conf := &Config{
		BaseURL:         getenvOr("SEAEYE_BASEURL", configDefaultBaseURL),
		GithubToken:     os.Getenv("SEAEYE_GITHUB_TOKEN"),
		HookbotEndpoint: os.Getenv("SEAEYE_HOOKBOT_ENDPOINT"),
		HostPort:        getenvOr("SEAEYE_PORT", configDefaultHostPort),
	}

	return conf
}

func getenvOr(k, fallback string) string {
	// if v, ok := os.LookupEnv("k"); ok {
	if v := os.Getenv("k"); v != "" {
		return v
	}
	return fallback
}
