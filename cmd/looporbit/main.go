package main

import (
	"looporbit/internal/cli"
	"os"
)

func main() {
	// cmd/looporbit 是正式入口，当前只启动纯 TUI 界面壳。
	os.Exit(cli.Run(os.Args[1:]))
}
