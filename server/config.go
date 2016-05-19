package seaeye

import (
	"log"
	"net"
	"os"
)

const (
	configDefaultHostPort  = ":19515" // "SEAE"(YE)
	configDefaultURLPrefix = "http://localhost:19515"
)

// Config specifies the configuration to run the seaeye server.
type Config struct {
	// HostPort holds Seaeye's server host and port.
	HostPort string
	// URLPrefix holds Seaeye's link scheme, authority, and port.
	URLPrefix string
}

// NewConfig creates a new configuration with a mix of default values and
// provided environment variables.
func NewConfig() *Config {
	log.Println("Info: [config] Loading configuration")

	conf := &Config{
		HostPort:  configDefaultHostPort,
		URLPrefix: configDefaultURLPrefix,
	}

	if port := os.Getenv("SEAEYE_PORT"); port != "" {
		conf.HostPort = net.JoinHostPort("", port)
	}

	if urlPrefix := os.Getenv("SEAEYE_URLPREFIX"); urlPrefix != "" {
		conf.URLPrefix = urlPrefix
	}

	return conf
}
