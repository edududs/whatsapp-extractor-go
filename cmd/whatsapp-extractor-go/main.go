package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/edududs/whatsapp-extractor-go/internal/cli"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, version, os.LookupEnv); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
