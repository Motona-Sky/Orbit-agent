package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"orbit/internal/config"
	"orbit/internal/tools"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

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
func McpConfigPaths() ([]string, error) {
	configPaths, err := config.GetConfigFolderPath()
	if err != nil {
		return nil, fmt.Errorf("resolve global config folder: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve project working directory: %w", err)
	}
	return []string{
		filepath.Join(configPaths["ConfigFolder"], ".mcp.json"),
		filepath.Join(cwd, ".mcp.json"),
	}, nil
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
	paths, err := McpConfigPaths()
	if err != nil {
		return MCPConfig{}, err
	}

	for _, path := range paths {
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
	Name:    "orbit-agent",
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
	base   http.RoundTripper
	header http.Header
	origin *url.URL
}

func sameOrigin(origin, target *url.URL) bool {
	return origin != nil && target != nil &&
		strings.EqualFold(origin.Scheme, target.Scheme) &&
		strings.EqualFold(origin.Host, target.Host)
}

func (t *headerInjectingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !sameOrigin(t.origin, req.URL) {
		return nil, fmt.Errorf("refuse cross-origin MCP request from %s to %s", t.origin, req.URL)
	}
	cloned := req.Clone(req.Context())
	for key, values := range t.header {
		for _, value := range values {
			cloned.Header.Add(key, value)
		}
	}
	return t.base.RoundTrip(cloned)
}

func newHTTPClientWithHeaders(endpoint string, headers map[string]string) (*http.Client, error) {
	origin, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse MCP endpoint %q: %w", endpoint, err)
	}
	if origin.Scheme == "" || origin.Host == "" {
		return nil, fmt.Errorf("MCP endpoint %q must be an absolute URL", endpoint)
	}

	header := make(http.Header, len(headers))
	for key, value := range headers {
		header.Set(key, value)
	}
	return &http.Client{
		Transport: &headerInjectingTransport{
			base:   http.DefaultTransport,
			header: header,
			origin: origin,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !sameOrigin(origin, req.URL) {
				return fmt.Errorf("refuse cross-origin MCP redirect from %s to %s", origin, req.URL)
			}
			return nil
		},
	}, nil
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

	httpClient, err := newHTTPClientWithHeaders(config.URL, config.Headers)
	if err != nil {
		return nil, nil, err
	}
	transport := &mcpsdk.StreamableClientTransport{
		Endpoint:   config.URL,
		HTTPClient: httpClient,
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

	httpClient, err := newHTTPClientWithHeaders(config.URL, config.Headers)
	if err != nil {
		return nil, nil, err
	}
	transport := &mcpsdk.SSEClientTransport{
		Endpoint:   config.URL,
		HTTPClient: httpClient,
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
	ClearMcpState()
	config, err := LoadMcpconfig()
	if err != nil {
		return nil, fmt.Errorf("load mcp config: %w", err)
	}

	var clients []*McpClient
	var errs []error
	for name, server := range config.MCPServers {
		client, serverTools, err := runMcpByType(ctx, server)
		if err != nil {
			errs = append(errs, fmt.Errorf("start mcp server %q: %w", name, err))
			continue
		}
		if _, err := RegisterMcpTools(name, client, serverTools); err != nil {
			_ = client.Close()
			errs = append(errs, fmt.Errorf("register mcp server %q: %w", name, err))
			continue
		}
		clients = append(clients, client)
	}
	mcpStateMu.Lock()
	activeClients = append([]*McpClient(nil), clients...)
	mcpStateMu.Unlock()

	if len(errs) > 0 {
		return clients, errors.Join(errs...)
	}
	return clients, nil
}

// === MCP 工具注册表 ============================================================
//
// McpToolRegistry 保存所有已注册的 MCP 工具，key 是暴露给模型的工具名。
// SelectAndCallMcp 依赖此表把模型调用转发回对应的 MCP server。
// 同时 RegisterMcpTools 会把每个工具登记到 tools.RegMcpToolFuncs 和 tools 的 schema
// 注册表，让 MCP 工具与本地工具在 agent 主循环里走同一条 RunTools 路径，
// 不再需要 SplitMcpToolCalls 之类的分流逻辑。

// McpToolEntry 描述一个暴露给模型的 MCP 工具：由哪个 client 处理、原始工具名是什么。
type McpToolEntry struct {
	Client      *McpClient // 拥有该工具的 MCP client session
	Server      string     // 所属 MCP server 名称（用于日志/前缀）
	ToolName    string     // MCP server 上的原始工具名（转发 CallTool 时使用）
	Description string     // 工具描述，原样回填给模型
	InputSchema any        // 原始 JSON Schema，原样回填给模型的 parameters 字段
}

var (
	McpToolRegistry = map[string]McpToolEntry{}
	mcpStateMu      sync.Mutex
	activeClients   []*McpClient
	invalidNameRun  = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
)

func ExposedToolName(server, tool string) string {
	digest := sha256.Sum256([]byte(server + "\x00" + tool))
	suffix := hex.EncodeToString(digest[:6])
	base := "mcp_" + sanitizeToolName(server) + "_" + sanitizeToolName(tool)
	maxBase := 64 - 1 - len(suffix)
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	base = strings.Trim(base, "_-")
	if base == "" {
		base = "mcp"
	}
	return base + "_" + suffix
}

func sanitizeToolName(value string) string {
	value = invalidNameRun.ReplaceAllString(value, "_")
	return strings.Trim(value, "_-")
}

func RegisterMcpTools(serverName string, client *McpClient, mcpTools []*mcpsdk.Tool) ([]string, error) {
	type pendingTool struct {
		name   string
		entry  McpToolEntry
		fn     tools.ToolFunc
		schema tools.ToolReg
	}
	pending := make([]pendingTool, 0, len(mcpTools))
	seen := make(map[string]struct{}, len(mcpTools))

	mcpStateMu.Lock()
	defer mcpStateMu.Unlock()
	for _, tool := range mcpTools {
		if tool == nil {
			return nil, fmt.Errorf("MCP server %q returned a nil tool", serverName)
		}
		exposed := ExposedToolName(serverName, tool.Name)
		if _, ok := seen[exposed]; ok {
			return nil, fmt.Errorf("duplicate MCP tool name %q in server %q", exposed, serverName)
		}
		if tools.HasLocalTool(exposed) {
			return nil, fmt.Errorf("MCP tool name %q conflicts with a local tool", exposed)
		}
		if _, ok := McpToolRegistry[exposed]; ok {
			return nil, fmt.Errorf("MCP tool name %q is already registered", exposed)
		}
		if _, ok := tools.RegMcpToolFuncs[exposed]; ok {
			return nil, fmt.Errorf("MCP tool function %q is already registered", exposed)
		}
		seen[exposed] = struct{}{}
		exposedName := exposed
		pending = append(pending, pendingTool{
			name: exposed,
			entry: McpToolEntry{
				Client:      client,
				Server:      serverName,
				ToolName:    tool.Name,
				Description: tool.Description,
				InputSchema: tool.InputSchema,
			},
			fn: tools.ToolFunc{
				Name: exposed,
				Function: func(ctx context.Context, args []any) (string, error) {
					_, result, err := SelectAndCallMcp(ctx, exposedName, mcpArgsToJSON(args))
					return result, err
				},
			},
			schema: tools.ToolReg{
				Type: "function",
				Function: tools.ToolFunction{
					Name:        exposed,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			},
		})
	}

	names := make([]string, 0, len(pending))
	schemas := make([]tools.ToolReg, 0, len(pending))
	for _, item := range pending {
		McpToolRegistry[item.name] = item.entry
		tools.RegMcpToolFuncs[item.name] = item.fn
		names = append(names, item.name)
		schemas = append(schemas, item.schema)
	}
	tools.RegMcpTools(schemas)
	return names, nil
}

func ClearMcpState() {
	mcpStateMu.Lock()
	clients := activeClients
	activeClients = nil
	McpToolRegistry = map[string]McpToolEntry{}
	tools.ClearMcpTools()
	mcpStateMu.Unlock()
	for _, client := range clients {
		_ = client.Close()
	}
}

func RegisteredMcpToolNames() []string {
	mcpStateMu.Lock()
	defer mcpStateMu.Unlock()
	names := make([]string, 0, len(McpToolRegistry))
	for name := range McpToolRegistry {
		names = append(names, name)
	}
	return names
}

// mcpArgsToJSON 把 ToolFunc 收到的 []any 还原为 arguments JSON 字符串。
// RunTools 调用 Function 时传入 []any{function}，其中 function["arguments"] 是模型返回的 JSON 字符串。
// 与 CallGrepFunc/CallReadFIleFunc 等本地工具的解析约定一致。
func mcpArgsToJSON(args []any) string {
	if len(args) == 0 {
		return ""
	}
	function, ok := args[0].(map[string]any)
	if !ok {
		return ""
	}
	arguments, _ := function["arguments"].(string)
	return arguments
}

// === 选择 MCP 服务器并发起调用 ================================================
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
	mcpStateMu.Lock()
	entry, ok := McpToolRegistry[exposedName]
	mcpStateMu.Unlock()
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
