// Command portlens is a local port intelligence and process management CLI.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/portlens/portlens/cmd"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	os.Exit(cmd.ExecuteContext(ctx, os.Args[1:], os.Stdout, os.Stderr, os.Stdin))
}
