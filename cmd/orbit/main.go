package main

import (
	"orbit/internal/cli"
	"os"
)
# cpvs6666
func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
