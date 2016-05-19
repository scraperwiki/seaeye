package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/scraperwiki/seaeye/server"
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
	a := &seaeye.App{Config: seaeye.NewConfig()}
	log.Println("Info: [cmd] Starting")
	if err := a.Start(); err != nil {
		os.Exit(1)
	}
	log.Println("Info: [cmd] Stopping")
	if err := a.Stop(); err != nil {
		os.Exit(1)
	}
}
