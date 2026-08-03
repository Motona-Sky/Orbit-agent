package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"looporbit/internal/utils"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
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
type McpClient struct {
	Session *mcpsdk.ClientSession
}

func (c *McpClient) Close() error {
	return c.Session.Close()
}

// McpConfigPaths 返回所有需要读取的 .mcp.json 路径，按优先级从低到高排列。
// 后面的路径在合并时会覆盖前面同名的 server（项目级配置优先于全局配置）。
func McpConfigPaths() []string {
	return []string{
		filepath.Join(utils.ConfigFolderPath, ".mcp.json"), // 全局配置（可选）
		filepath.Join(utils.Cwd, ".mcp.json"),              // 项目配置（优先级更高）
	}
}

// loadMcpConfigFile 读取单个路径下的 .mcp.json。
// 文件不存在时返回空配置而非错误，因为并非每个路径都必须存在。
func loadMcpConfigFile(path string) (MCPConfig, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MCPConfig{}, nil
		}
		return MCPConfig{}, fmt.Errorf("read mcp config %q: %w", path, err)
	}

	var config MCPConfig
	if err := json.Unmarshal(file, &config); err != nil {
		return MCPConfig{}, fmt.Errorf("parse mcp config %q: %w", path, err)
	}
	return config, nil
}

// LoadMcpconfig 从多个路径读取 .mcp.json 并合并去重。
// 去重依据 server name：同名 server 以优先级更高的路径（列表中更靠后的路径）为准。
func LoadMcpconfig() (MCPConfig, error) {
	merged := MCPConfig{MCPServers: make(map[string]MCP)}

	for _, path := range McpConfigPaths() {
		config, err := loadMcpConfigFile(path)
		if err != nil {
			return MCPConfig{}, err
		}
		for name, server := range config.MCPServers {
			merged.MCPServers[name] = server // 同名覆盖，天然去重
		}
	}

	return ParseMcp(merged), nil
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

// newMcpClientImplementation 是所有 MCP client 连接时使用的统一身份标识。
var newMcpClientImplementation = &mcpsdk.Implementation{
	Name:    "looporbit-agent",
	Version: "dev",
}

// newClientSession 用统一的 client identity 连接到任意 transport，
// 抽出来避免 RunStdioMcp/RunHTTPMcp/RunSSEMcp 重复构造 client。
func newClientSession(ctx context.Context, transport mcpsdk.Transport) (*mcpsdk.ClientSession, error) {
	client := mcpsdk.NewClient(newMcpClientImplementation, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect MCP server: %w", err)
	}
	return session, nil
}

// listToolsFromSession 从已连接的 session 拉取全部工具列表。
// 出错时关闭 session 并返回错误，与旧版 RunStdioMcp 行为一致。
func listToolsFromSession(ctx context.Context, session *mcpsdk.ClientSession) ([]*mcpsdk.Tool, error) {
	var tools []*mcpsdk.Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			_ = session.Close()
			return nil, fmt.Errorf("list MCP tools: %w", err)
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

// headerInjectingTransport 是一个 http.RoundTripper，在每个请求上附加固定的请求头。
// 用于把 .mcp.json 中配置的 Headers（如 Authorization）注入到 HTTP/SSE 传输中，
// 因为 SDK 的 StreamableClientTransport/SSEClientTransport 都不直接支持自定义 headers。
type headerInjectingTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerInjectingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// RoundTripper 约定不能修改传入的请求，需先 Clone。
	cloned := req.Clone(req.Context())
	for key, value := range t.headers {
		cloned.Header.Set(key, value)
	}
	return t.base.RoundTrip(cloned)
}

// newHTTPClientWithHeaders 返回一个在每次请求中注入固定 headers 的 *http.Client。
// headers 为空时返回 nil，调用方应据此使用 SDK 默认的 http.DefaultClient。
func newHTTPClientWithHeaders(headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return nil
	}
	return &http.Client{
		Transport: &headerInjectingTransport{
			base:    http.DefaultTransport,
			headers: headers,
		},
	}
}

// RunStdioMcp 启动 stdio MCP Server，完成初始化并获取全部工具列表。
// 调用方应在不再使用 client 时调用 client.Close。
func RunStdioMcp(ctx context.Context, config MCP) (*McpClient, []*mcpsdk.Tool, error) {
	if config.Command == "" {
		return nil, nil, fmt.Errorf("command is empty")
	}

	cmd := exec.CommandContext(ctx, config.Command, config.Args...)
	cmd.Env = os.Environ()
	for key, value := range config.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.Stderr = os.Stderr

	session, err := newClientSession(ctx, &mcpsdk.CommandTransport{Command: cmd})
	if err != nil {
		return nil, nil, err
	}

	tools, err := listToolsFromSession(ctx, session)
	if err != nil {
		return nil, nil, err
	}

	return &McpClient{Session: session}, tools, nil
}

// RunHTTPMcp 连接到 streamable HTTP MCP server（MCP 2025-03-26+ 协议）。
// config.URL 必填；config.Headers 会作为固定请求头注入（例如 Authorization）。
// 调用方应在不再使用 client 时调用 client.Close。
func RunHTTPMcp(ctx context.Context, config MCP) (*McpClient, []*mcpsdk.Tool, error) {
	if config.URL == "" {
		return nil, nil, fmt.Errorf("url is empty")
	}

	transport := &mcpsdk.StreamableClientTransport{
		Endpoint:   config.URL,
		HTTPClient: newHTTPClientWithHeaders(config.Headers),
	}

	session, err := newClientSession(ctx, transport)
	if err != nil {
		return nil, nil, err
	}

	tools, err := listToolsFromSession(ctx, session)
	if err != nil {
		return nil, nil, err
	}

	return &McpClient{Session: session}, tools, nil
}

// RunSSEMcp 连接到 legacy SSE MCP server（MCP 2024-11-05 协议）。
// config.URL 必填；config.Headers 会作为固定请求头注入（例如 Authorization）。
// 调用方应在不再使用 client 时调用 client.Close。
func RunSSEMcp(ctx context.Context, config MCP) (*McpClient, []*mcpsdk.Tool, error) {
	if config.URL == "" {
		return nil, nil, fmt.Errorf("url is empty")
	}

	transport := &mcpsdk.SSEClientTransport{
		Endpoint:   config.URL,
		HTTPClient: newHTTPClientWithHeaders(config.Headers),
	}

	session, err := newClientSession(ctx, transport)
	if err != nil {
		return nil, nil, err
	}

	tools, err := listToolsFromSession(ctx, session)
	if err != nil {
		return nil, nil, err
	}

	return &McpClient{Session: session}, tools, nil
}

// runMcpByType 根据 config.Type 分发到对应的 Run* 函数。
// 类型识别与 ParseMcp 的默认值规则对齐（LoadMcpconfig 已保证 Type 非空）。
func runMcpByType(ctx context.Context, config MCP) (*McpClient, []*mcpsdk.Tool, error) {
	switch config.Type {
	case "stdio", "":
		return RunStdioMcp(ctx, config)
	case "http":
		return RunHTTPMcp(ctx, config)
	case "sse":
		return RunSSEMcp(ctx, config)
	default:
		return nil, nil, fmt.Errorf("unsupported MCP type: %s", config.Type)
	}
}

// RunMcp 是运行全部 MCP server 的统一入口：
//  1. 从多个路径加载并合并 .mcp.json 配置
//  2. 逐个 server 按 Type 启动并连接
//  3. 把每个 server 暴露的工具注册到 McpToolRegistry，供模型调用
//
// 单个 server 启动失败不会中断整体流程，会跳过该 server 并把错误收集到返回值中，
// 便于上层决定是否继续（例如仅记录日志，不影响其它可用的 MCP server）。
//
// 调用方在 Agent 结束时应对返回的每个 client 调用 Close。
func RunMcp(ctx context.Context) ([]*McpClient, error) {
	config, err := LoadMcpconfig()
	if err != nil {
		return nil, fmt.Errorf("load mcp config: %w", err)
	}

	var clients []*McpClient
	var errs []error
	for name, server := range config.MCPServers {
		client, tools, err := runMcpByType(ctx, server)
		if err != nil {
			errs = append(errs, fmt.Errorf("start mcp server %q: %w", name, err))
			continue
		}
		RegisterMcpTools(name, client, tools)
		clients = append(clients, client)
	}

	if len(errs) > 0 {
		return clients, errors.Join(errs...)
	}
	return clients, nil
}

// === MCP 工具注册表 ============================================================
//
// McpToolRegistry 保存所有已注册的 MCP 工具，key 是暴露给模型的工具名。
// SelectAndCallMcp 和 SplitMcpToolCalls 都依赖此表来识别 MCP 工具。
// 由 RegisterMcpTools 在 Agent 启动时填充。

// McpToolEntry 描述一个暴露给模型的 MCP 工具：由哪个 client 处理、原始工具名是什么。
type McpToolEntry struct {
	Client   *McpClient // 拥有该工具的 MCP client session
	Server   string     // 所属 MCP server 名称（用于日志/前缀）
	ToolName string     // MCP server 上的原始工具名（转发 CallTool 时使用）
}

var McpToolRegistry = map[string]McpToolEntry{}

// ExposedToolName 生成暴露给模型的工具名：mcp__<server>__<tool>。
// 该前缀用于区分 MCP 工具与本地工具，并避免不同 server 间的同名冲突。
func ExposedToolName(server, tool string) string {
	return "mcp__" + server + "__" + tool
}

// RegisterMcpTools 把一个 MCP server 暴露的全部工具登记到 McpToolRegistry。
// 返回所有暴露给模型的工具名，便于上层合并到 tools 列表传给模型。
func RegisterMcpTools(serverName string, client *McpClient, mcpTools []*mcpsdk.Tool) []string {
	names := make([]string, 0, len(mcpTools))
	for _, t := range mcpTools {
		exposed := ExposedToolName(serverName, t.Name)
		McpToolRegistry[exposed] = McpToolEntry{
			Client:   client,
			Server:   serverName,
			ToolName: t.Name,
		}
		names = append(names, exposed)
	}
	return names
}

// IsMcpTool 判断暴露给模型的工具名是否属于已注册的 MCP 工具。
func IsMcpTool(exposedName string) bool {
	_, ok := McpToolRegistry[exposedName]
	return ok
}

// === 函数 1：选择 MCP 服务器并发起调用 =========================================
//
// SelectAndCallMcp 根据暴露给模型的工具名，从注册表中找到对应的 MCP server，
// 解析模型返回的 arguments JSON 字符串，转发给 MCP server 的 CallTool。
//
// 参数：
//   - ctx:          调用上下文，用于超时/取消控制
//   - exposedName:  模型返回的 tool_calls 中的工具名（mcp__server__tool 形式）
//   - argumentsJSON: 模型返回的 arguments 字段（JSON 字符串，可能为 ""）
//
// 返回值：
//   - handled: 是否由 MCP 处理。false 表示该工具名不属于 MCP，应由本地工具链处理。
//   - result:  调用结果文本，用于回填到 role=tool 的消息内容。
//   - err:     调用过程中的错误（仅当 handled=true 时有意义）。
//
// 并发安全：可在多个 goroutine 中同时调用（同一 session 的 CallTool 由 SDK
// 通过 JSON-RPC request ID 多路复用）。
func SelectAndCallMcp(ctx context.Context, exposedName, argumentsJSON string) (handled bool, result string, err error) {
	entry, ok := McpToolRegistry[exposedName]
	if !ok {
		return false, "", nil
	}

	var args map[string]any
	if argumentsJSON != "" {
		if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
			return true, "", fmt.Errorf("parse mcp arguments for %q: %w", exposedName, err)
		}
	}

	res, callErr := entry.Client.Session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      entry.ToolName, // 转发给 MCP server 时用原始名，不是暴露名
		Arguments: args,
	})
	if callErr != nil {
		return true, "", fmt.Errorf("call mcp %s/%s: %w", entry.Server, entry.ToolName, callErr)
	}
	return true, mcpResultToString(res), nil
}

// mcpResultToString 把 CallToolResult.Content 拼成纯文本，便于回填到 tool 角色消息。
// 非 TextContent（image/audio 等）退化为 JSON 文本。
func mcpResultToString(res *mcpsdk.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			sb.WriteString(tc.Text)
			continue
		}
		b, _ := json.Marshal(c)
		sb.Write(b)
	}
	if res.IsError {
		return "MCP tool error: " + sb.String()
	}
	return sb.String()
}

// === 函数 2：从模型返回的 tool_calls 中分离 MCP 调用 =============================
//
// SplitMcpToolCalls 把模型返回的 tool_calls 切分为 MCP 调用与本地调用两组。
// 每条记录都保留在原 tool_calls 中的下标 Index，便于调用方在并行执行后按原顺序合并结果。
//
// 输入格式参考 ParseResponse：[]any，每个元素形如
//
//	map[id:"call_xxx" function:map[arguments:"{\"k\":\"v\"}" name:"mcp__time__now"] type:"function"]
//
// 返回：
//   - mcpCalls:   所有属于 MCP 工具的调用（保留原下标）
//   - localCalls: 所有不属于 MCP 工具的调用（保留原下标，原样传给 tools.RunTools）
//   - err:        解析失败
func SplitMcpToolCalls(toolCalls []any) (mcpCalls []McpToolCall, localCalls []LocalToolCall, err error) {
	for i, value := range toolCalls {
		toolCall, ok := value.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("tool_call[%d] is not map[string]any", i)
		}
		id, _ := toolCall["id"].(string)
		function, ok := toolCall["function"].(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("tool_call[%d] missing function", i)
		}
		name, _ := function["name"].(string)
		arguments, _ := function["arguments"].(string)

		if IsMcpTool(name) {
			mcpCalls = append(mcpCalls, McpToolCall{
				Index:         i,
				ToolCallID:    id,
				ExposedName:   name,
				ArgumentsJSON: arguments,
			})
		} else {
			localCalls = append(localCalls, LocalToolCall{
				Index: i,
				Raw:   toolCall,
			})
		}
	}
	return mcpCalls, localCalls, nil
}

// McpToolCall 表示一个属于 MCP 的工具调用，保留原下标以便按顺序合并结果。
type McpToolCall struct {
	Index         int    // 在模型原始 tool_calls 数组中的下标
	ToolCallID    string // 模型返回的 id（如 call_xxx），用于回填 tool 角色消息
	ExposedName   string // 暴露给模型的工具名（mcp__server__tool）
	ArgumentsJSON string // 模型返回的 arguments（JSON 字符串）
}

// LocalToolCall 表示属于本地工具链的调用，原样传给 tools.RunTools。
type LocalToolCall struct {
	Index int
	Raw   map[string]any
}
