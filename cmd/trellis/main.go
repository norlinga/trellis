package main

import (
	"fmt"
	"os"

	"github.com/norlinga/trellis/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "trellis:", err)
		os.Exit(1)
	}
}
