package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/skndan/sentry-lite/internal/load"
)

func main() {
	cfg := load.DefaultConfig()
	fs := flag.NewFlagSet("sentry-lite-load", flag.ExitOnError)
	cfg.RegisterFlags(fs)
	_ = fs.Parse(os.Args[1:])

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Headless {
		if err := load.RunHeadless(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "load: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := load.RunTUI(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}
}
