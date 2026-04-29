package main

import (
	"fmt"
	"os"

	"github.com/oxhq/ctx/internal/cli"
)

var version = "dev"

func main() {
	cli.Version = version
	if err := cli.ExecuteWithOutput(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
