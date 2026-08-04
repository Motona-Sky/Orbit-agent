package main

import (
	"orbit/internal/cli"
	"os"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
