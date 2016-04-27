package main

import (
	"log"
	"os"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/codegangsta/cli"
	"github.com/scraperwiki/seaeye"
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
	conf := seaeye.NewConfig()
	seaeye.Start(conf)
}
