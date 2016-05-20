package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/scraperwiki/seaeye/pkg"
)

var version string

func main() {
	var versionFlag bool

	flag.BoolVar(&versionFlag, "v", false, "Show version information and exit")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: seaeye [OPTION]...")
		fmt.Fprintln(os.Stderr, "Simple continuous integration server.")
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
	}
	flag.Parse()

	if versionFlag {
		fmt.Println("seaeye", version)
		return
	}

	mainCmd()
}

func mainCmd() {
	config := seaeye.NewConfig()
	config.Version = version

	a := &seaeye.App{Config: config}

	log.Println("[I][cmd] Starting")
	if err := a.Start(); err != nil {
		os.Exit(1)
	}

	log.Println("[I][cmd] Stopping")
	if err := a.Stop(); err != nil {
		os.Exit(1)
	}
}
