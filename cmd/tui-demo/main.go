package main

import (
	"fmt"
	"os"

	"orbit/internal/cli"
)

func main() {
	if _, _, err := cli.OpenLanTui(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
