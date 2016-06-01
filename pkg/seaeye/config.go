package seaeye

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	configDefaultHostPort     = ":19515" // "SEAE"(YE)
	configDefaultBaseURL      = "http://localhost:19515"
	configDefaultLogBaseDir   = "logs"
	configDefaultFetchBaseDir = "workspace"
	configDefaultExecTimeout  = 1 * time.Hour
)

// Config specifies the configuration to run the seaeye application.
type Config struct {
	// BaseURL holds Seaeye's link scheme, authority, and port.
	BaseURL string
	// ExecTimeout defines the timeout after which test execution steps are
	// canceled.
	ExecTimeout time.Duration
	// FetchBaseDir defines the base directory to any fetched source files.
	FetchBaseDir string
	// GithubToken holds a Personal Access Token for Github to authenticate
	// commit status updates via Github API.
	GithubToken string
	// HookbotEndpoint holds a hookbot subscription URL.
	HookbotEndpoint string
	// HostPort holds Seaeye's server host and port.
	HostPort string
	// LogBaseDir defines the base directory to log files.
	LogBaseDir string
	// Seaeye version
	Version string
}

// NewConfig creates a new configuration with a mix of default values and
// provided environment variables.
func NewConfig() *Config {
	log.Println("[I][config] Loading configuration")

	conf := &Config{
		BaseURL:      configDefaultBaseURL,
		HostPort:     configDefaultHostPort,
		LogBaseDir:   configDefaultLogBaseDir,
		FetchBaseDir: configDefaultFetchBaseDir,
		ExecTimeout:  configDefaultExecTimeout,
	}

	if v, ok := os.LookupEnv("SEAEYE_BASEURL"); ok {
		conf.BaseURL = v
	}
	if v, ok := os.LookupEnv("SEAEYE_GITHUB_TOKEN"); ok {
		conf.GithubToken = v
	}
	if v, ok := os.LookupEnv("SEAEYE_HOOKBOT_ENDPOINT"); ok {
		conf.HookbotEndpoint = v
	}
	if v, ok := os.LookupEnv("SEAEYE_HOSTPORT"); ok {
		conf.HostPort = v
	}
	if v, ok := os.LookupEnv("SEAEYE_LOG_BASEDIR"); ok {
		conf.LogBaseDir = v
	}
	if v, ok := os.LookupEnv("SEAEYE_FETCH_BASEDIR"); ok {
		conf.FetchBaseDir = v
	}
	if v, ok := os.LookupEnv("SEAEYE_EXEC_TIMEOUT"); ok {
		if d, err := time.ParseDuration(v); err != nil {
			log.Printf("[W][config] Failed to parse SEAEYE_EXEC_TIMEOUT: %v", err)
		} else {
			conf.ExecTimeout = d
		}
	}

	return conf
}

// LogFilePath assembles a log file path from a job id and revision.
func (c *Config) LogFilePath(jobID, rev string) (string, error) {
	saneID := escapePath(jobID) // e.g.: scraperwiki/foo
	saneRev := escapePath(rev)  // e.g.: refs/origin/master
	filePath := path.Join(c.LogBaseDir, saneID, saneRev, "log.txt")
	return filepath.Abs(filePath)
}

func escapePath(path string) string {
	p := path
	p = strings.Replace(p, "/", "_", -1)
	p = strings.Replace(p, ":", "_", -1)
	return p
}
