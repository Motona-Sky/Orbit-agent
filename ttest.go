package main

import (
	"context"
	"fmt"
	"looporbit/internal/mcp"
)

func main() {
	// RunMcp 是获取 MCP 工具的统一入口：
	// 1. 从多路径加载并合并 .mcp.json
	// 2. 按 Type（stdio/http/sse）逐个启动 server
	// 3. 把每个 server 的工具注册进 mcp.McpToolRegistry
	clients, err := mcp.RunMcp(context.Background())
	if err != nil {
		// RunMcp 对单个 server 失败是容错的（跳过继续），
		// 这里的 err 汇总了所有失败的 server，仅打印不中断。
		fmt.Println("some mcp servers failed to start:", err)
	}
	defer func() {
		for _, c := range clients {
			_ = c.Close()
		}
	}()

	// 工具列表来自 McpToolRegistry：key 是暴露给模型的工具名（mcp__server__tool）。
	for exposedName, entry := range mcp.McpToolRegistry {
		fmt.Printf("%s (server=%s, tool=%s)\n", exposedName, entry.Server, entry.ToolName)
	}
}
