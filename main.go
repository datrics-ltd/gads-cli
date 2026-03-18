package main

import (
	"os"

	"github.com/datrics-ltd/gads-cli/cmd"
)

// version is set at build time via -ldflags "-X main.version=v1.2.3"
var version = "dev"

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
