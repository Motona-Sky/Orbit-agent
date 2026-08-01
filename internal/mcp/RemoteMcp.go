package mcp

import (
	"encoding/json"
	"fmt"
	"looporbit/internal/utils"
	"os"
	"path/filepath"
)

type MCPConfig struct {
	MCPServers map[string]MCP `json:"mcpServers"`
}

type MCP struct {
	Type    string            `json:"type"`    // 传输类型: "stdio" | "http" | "sse"
	Command string            `json:"command"` // 本地 stdio 服务的启动命令 (如 "node", "npx", "uvx")
	Args    []string          `json:"args"`    // 传递给 command 的参数列表
	Env     map[string]string `json:"env"`     // 环境变量映射表
	URL     string            `json:"url"`     // 远程 HTTP / SSE 服务的连接地址
	Headers map[string]string `json:"headers"` // HTTP 请求头 (可放 Auth 认证信息)
}
type stdioMcp struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}
type httpMcp struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// LoadMcpconfig 从当前工作目录读取 .mcp.json。
func LoadMcpconfig() (MCPConfig, error) {
	path := filepath.Join(utils.Cwd, ".mcp.json")
	file, err := os.ReadFile(path)
	if err != nil {
		return MCPConfig{}, err
	}

	var config MCPConfig
	if err := json.Unmarshal(file, &config); err != nil {
		return MCPConfig{}, err
	}
	mcps := ParseMcp(config)
	return mcps, nil
}
func ParseMcp(config MCPConfig) MCPConfig {
	for name, server := range config.MCPServers {
		if server.Type == "" {
			server.Type = "stdio"
			config.MCPServers[name] = server
		}
	}

	return config
}
func RunMcp(config MCP) error {
	switch config.Type {
	case "stdio":
		return runStdioMcp(config)
	case "http":
		return runHttpMcp(config)
	case "sse":
		return runSseMcp(config)
	default:
		return fmt.Errorf("unsupported MCP type: %s", config.Type)
	}
	return nil
}
