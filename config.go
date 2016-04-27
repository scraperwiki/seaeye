package seaeye

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

const (
	configDefaultPort = 19515 // "SEAE"(YE)
)

// Config specifies the configuration to run the seaeye server.
type Config struct {
	// Port holds Seaeye's server port.
	Port int
	// URLAuthority holds Seaeye's link authority.
	URLAuthority string

	// HookbotEndpoint holds Hookbot's subscribe endpoint.
	HookbotEndpoint string

	// GithubUser holds Github API username.
	GithubUser string
	// GithubToken holds Github API token.
	GithubToken string
}

// NewConfig creates a new configuration with a mix of default values and
// provided environment variables.
func NewConfig() *Config {
	conf := &Config{
		Port:         configDefaultPort,
		URLAuthority: "localhost",
	}

	if v := os.Getenv("SEAEYE_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("Error: Failed to parse specified port: %v", err)
		}
		conf.Port = port
	}

	if v := os.Getenv("SEAEYE_URL_AUTHORITY"); v != "" {
		conf.URLAuthority = v
	}

	if v := os.Getenv("HOOKBOT_SUB_ENDPOINT"); v != "" {
		conf.HookbotEndpoint = v
	}

	if v := os.Getenv("GITHUB_USER"); v != "" {
		conf.GithubUser = v
	}

	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		conf.GithubToken = v
	}

	urlPrefix := fmt.Sprintf("http://%s:%s", authority, port)
	baseDir, err := os.Getwd()
	if err != nil {
		log.Fatalln("Error:", err)
	}

}
