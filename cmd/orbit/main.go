package main

import (
	"orbit/internal/cli"
	"os"
)
// cpvs666
func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
