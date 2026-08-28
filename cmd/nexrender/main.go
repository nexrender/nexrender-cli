package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nexrender/nexrender-cli/internal/clierr"
	"github.com/nexrender/nexrender-cli/internal/command"
)

func main() {
	state, err := command.DefaultState()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(clierr.ExitServer)
	}
	os.Exit(command.Execute(context.Background(), state, os.Args[1:]))
}
