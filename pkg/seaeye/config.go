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
	defaultHostPort         = ":19515" // "SEAE"(YE)
	defaultBaseURL          = "http://localhost:19515"
	defaultHookbotEndpoint  = ""
	dockerHostVolumeBaseDir = ""
	defaultGithubToken      = ""
	defaultLogBaseDir       = "logs"
	defaultFetchBaseDir     = "workspace"
	defaultExecTimeout      = "1h"
	defaultNoNotify         = "false"

	internalEnvPrefix = "SEAEYE_"
)

// Config specifies the configuration to run the seaeye application.
type Config struct {
	// BaseURL holds Seaeye's link scheme, authority, and port.
	BaseURL string
	// DockerHostVolumeBaseDir holds the host's Docker volume path prefix. If
	// empty, it is assumed that either no volume was mounted on the host or
	// that the volume mount paths on the host and in the container are
	// identical.
	DockerHostVolumeBaseDir string
	// ExecTimeout holds the timeout after which test execution steps are
	// canceled.
	ExecTimeout time.Duration
	// FetchBaseDir holds the base directory to any fetched source files.
	FetchBaseDir string
	// GithubToken holds a Personal Access Token for Github to authenticate
	// commit status updates via Github API.
	GithubToken string
	// HookbotEndpoint holds a hookbot subscription URL.
	HookbotEndpoint string
	// HostPort holds Seaeye's server host and port.
	HostPort string
	// LogBaseDir holds the base directory to log files.
	LogBaseDir string
	// NoNotify decides if webhook notifications are sent.
	NoNotify bool
	// Seaeye version
	Version string
}

// NewConfig creates a new configuration with a mix of default values and
// provided environment variables.
func NewConfig() *Config {
	log.Println("[I][config] Loading configuration")

	return &Config{
		BaseURL:                 getEnvOr("BASEURL", defaultBaseURL),
		DockerHostVolumeBaseDir: getEnvOr("DOCKER_VOL_BASEDIR", dockerHostVolumeBaseDir),
		ExecTimeout:             mustParseDuration(getEnvOr("EXEC_TIMEOUT", defaultExecTimeout)),
		FetchBaseDir:            getEnvOr("FETCH_BASEDIR", defaultFetchBaseDir),
		GithubToken:             getEnvOr("GITHUB_TOKEN", defaultGithubToken),
		HookbotEndpoint:         getEnvOr("HOOKBOT_ENDPOINT", defaultHookbotEndpoint),
		HostPort:                getEnvOr("HOSTPORT", defaultHostPort),
		LogBaseDir:              getEnvOr("LOG_BASEDIR", defaultLogBaseDir),
		NoNotify:                parseBool(getEnvOr("NO_NOTIFY", defaultNoNotify)),
	}
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(internalEnvPrefix + key); v != "" {
		return v
	}
	return fallback
}

func mustParseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Fatalf("[W][config] Failed to parse %s: %v", s, err)
	}
	return d
}

func parseBool(s string) bool {
	return s == "1" || strings.ToLower(s) == "true" || strings.ToLower(s) == "yes"
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
	p = filepath.Clean(p)
	return p
}
