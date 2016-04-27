package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/codegangsta/cli"
)

var version string

func init() {
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		log.SetPrefix("\x1b[34;1mseaeye\x1b[0m ")
	} else {
		log.SetPrefix("seaeye ")
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "seaeye"
	app.Usage = "CI server integrating Github and Hookbot."
	app.Version = version
	app.Action = ActionMain
	app.RunAndExitOnError()
}

// ActionMain is the main commandline command.
func ActionMain(c *cli.Context) {
	port := assertEnv("SEAEYE_PORT", "19515") // "SEAE"(YE)
	endpoint := assertEnv("HOOKBOT_SUB_ENDPOINT", "")
	user := assertEnv("GITHUB_USER", "")
	token := assertEnv("GITHUB_TOKEN", "")
	authority := assertEnv("CI_URL_AUTHORITY", "localhost")
	urlPrefix := fmt.Sprintf("http://%s:%s", authority, port)

	baseDir, err := os.Getwd()
	if err != nil {
		log.Fatalln("Error:", err)
	}

	Run(port, baseDir, user, token, urlPrefix, endpoint)
}

func assertEnv(key string, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		if fallback != "" {
			return fallback
		}
		log.Fatalln("Error:", key, "not set")
	}
	return val
}
