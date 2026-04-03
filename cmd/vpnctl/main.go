package main

import (
	"os"

	"github.com/BEDOLAGA-DEV/RemnaCore/cmd/vpnctl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
