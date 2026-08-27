// Command portlens is a local port intelligence and process management CLI.
package main

import (
	"os"

	"github.com/portlens/portlens/cmd"
)

func main() {
	os.Exit(cmd.Execute(os.Args[1:], os.Stdout, os.Stderr, os.Stdin))
}
