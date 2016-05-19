package main

import (
	"flag"
	"fmt"
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
	s := seaeye.New()
	if err := s.Start(); err != nil {
		os.Exit(1)
	}
	if err := s.Stop(); err != nil {
		os.Exit(1)
	}
}
