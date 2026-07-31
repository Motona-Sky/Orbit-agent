package mcp

type mcpServer struct {
	name mcp
}
type mcp struct {
	Type      string            `json:"type"`       // 传输类型: "stdio" | "http" | "sse"
	Command   string            `json:"command"`    // 本地 stdio 服务的启动命令 (如 "node", "npx", "uvx")
	Args      []string          `json:"args"`       // 传递给 command 的参数列表
	Env       map[string]string `json:"env"`        // 环境变量映射表
	URL       string            `json:"url"`        // 远程 HTTP / SSE 服务的连接地址
	Headers   map[string]string `json:"headers"`    // HTTP 请求头 (可放 Auth 认证信息)
	AutoStart *bool             `json:"auto_start"` // 控制是否随会话自动启动连接
}

// .mcp.json
func LoadMcpconfig() {

}
