package mcp

import (
	"bytes"
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
	"sort"
	"strings"
	"sync"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPConfig struct {
	MCPServers map[string]MCP `json:"mcpServers"`
}

type MCP struct {
	Type        string            `json:"type,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Description string            `json:"description,omitempty"`
}

func (m MCP) IsEnabled() bool { return m.Enabled == nil || *m.Enabled }

type McpClient struct {
	Session *mcpsdk.ClientSession
	once    sync.Once
	err     error
}

func (c *McpClient) Close() error {
	if c == nil {
		return nil
	}
	c.once.Do(func() {
		if c.Session != nil {
			c.err = c.Session.Close()
		}
	})
	return c.err
}

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

func loadMcpConfigFile(path string) (MCPConfig, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MCPConfig{}, nil
		}
		return MCPConfig{}, fmt.Errorf("read mcp config %q: %w", path, err)
	}
	var cfg MCPConfig
	if err := json.Unmarshal(file, &cfg); err != nil {
		return MCPConfig{}, fmt.Errorf("parse mcp config %q: %w", path, err)
	}
	return cfg, nil
}

func loadMcpConfigWithSources() (MCPConfig, map[string]string, error) {
	merged := MCPConfig{MCPServers: make(map[string]MCP)}
	sources := make(map[string]string)
	paths, err := McpConfigPaths()
	if err != nil {
		return MCPConfig{}, nil, err
	}
	for _, path := range paths {
		cfg, err := loadMcpConfigFile(path)
		if err != nil {
			return MCPConfig{}, nil, err
		}
		for name, server := range cfg.MCPServers {
			merged.MCPServers[name] = server
			sources[name] = path
		}
	}
	return ParseMcp(merged), sources, nil
}

func LoadMcpconfig() (MCPConfig, error) {
	cfg, _, err := loadMcpConfigWithSources()
	return cfg, err
}

func ParseMcp(cfg MCPConfig) MCPConfig {
	for name, server := range cfg.MCPServers {
		if server.Type == "" {
			server.Type = "stdio"
			cfg.MCPServers[name] = server
		}
	}
	return cfg
}

var newMcpClientImplementation = &mcpsdk.Implementation{Name: "orbit-agent", Version: "dev"}

func newClientSession(ctx context.Context, transport mcpsdk.Transport) (*mcpsdk.ClientSession, error) {
	client := mcpsdk.NewClient(newMcpClientImplementation, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect MCP server: %w", err)
	}
	return session, nil
}

func listToolsFromSession(ctx context.Context, session *mcpsdk.ClientSession) ([]*mcpsdk.Tool, error) {
	var result []*mcpsdk.Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list MCP tools: %w", err)
		}
		result = append(result, tool)
	}
	return result, nil
}

type headerInjectingTransport struct {
	base   http.RoundTripper
	header http.Header
	origin *url.URL
}

func sameOrigin(origin, target *url.URL) bool {
	return origin != nil && target != nil && strings.EqualFold(origin.Scheme, target.Scheme) && strings.EqualFold(origin.Host, target.Host)
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
		Transport: &headerInjectingTransport{base: http.DefaultTransport, header: header, origin: origin},
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if !sameOrigin(origin, req.URL) {
				return fmt.Errorf("refuse cross-origin MCP redirect from %s to %s", origin, req.URL)
			}
			return nil
		},
	}, nil
}

type recentBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func (b *recentBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	return len(p), nil
}

func (b *recentBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(bytes.TrimSpace(b.data)))
}

func runStdioMcp(ctx context.Context, cfg MCP, stderr *recentBuffer) (*McpClient, []*mcpsdk.Tool, error) {
	if cfg.Command == "" {
		return nil, nil, fmt.Errorf("command is empty")
	}
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Env = os.Environ()
	for key, value := range cfg.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.Stderr = stderr
	session, err := newClientSession(ctx, &mcpsdk.CommandTransport{Command: cmd})
	if err != nil {
		return nil, nil, err
	}
	client := &McpClient{Session: session}
	serverTools, err := listToolsFromSession(ctx, session)
	if err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	return client, serverTools, nil
}

func RunStdioMcp(ctx context.Context, cfg MCP) (*McpClient, []*mcpsdk.Tool, error) {
	return runStdioMcp(ctx, cfg, &recentBuffer{limit: 8192})
}

func RunHTTPMcp(ctx context.Context, cfg MCP) (*McpClient, []*mcpsdk.Tool, error) {
	if cfg.URL == "" {
		return nil, nil, fmt.Errorf("url is empty")
	}
	httpClient, err := newHTTPClientWithHeaders(cfg.URL, cfg.Headers)
	if err != nil {
		return nil, nil, err
	}
	session, err := newClientSession(ctx, &mcpsdk.StreamableClientTransport{Endpoint: cfg.URL, HTTPClient: httpClient})
	if err != nil {
		return nil, nil, err
	}
	client := &McpClient{Session: session}
	serverTools, err := listToolsFromSession(ctx, session)
	if err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	return client, serverTools, nil
}

func RunSSEMcp(ctx context.Context, cfg MCP) (*McpClient, []*mcpsdk.Tool, error) {
	if cfg.URL == "" {
		return nil, nil, fmt.Errorf("url is empty")
	}
	httpClient, err := newHTTPClientWithHeaders(cfg.URL, cfg.Headers)
	if err != nil {
		return nil, nil, err
	}
	session, err := newClientSession(ctx, &mcpsdk.SSEClientTransport{Endpoint: cfg.URL, HTTPClient: httpClient})
	if err != nil {
		return nil, nil, err
	}
	client := &McpClient{Session: session}
	serverTools, err := listToolsFromSession(ctx, session)
	if err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	return client, serverTools, nil
}

func runMcpByType(ctx context.Context, cfg MCP, stderr *recentBuffer) (*McpClient, []*mcpsdk.Tool, error) {
	switch cfg.Type {
	case "stdio", "":
		return runStdioMcp(ctx, cfg, stderr)
	case "http":
		return RunHTTPMcp(ctx, cfg)
	case "sse":
		return RunSSEMcp(ctx, cfg)
	default:
		return nil, nil, fmt.Errorf("unsupported MCP type: %s", cfg.Type)
	}
}

type ServiceState string

const (
	StateStarting ServiceState = "starting"
	StateReady    ServiceState = "ready"
	StateError    ServiceState = "error"
	StateDisabled ServiceState = "disabled"
	StateStopped  ServiceState = "stopped"
)

type ServiceStatus struct {
	Name        string
	Description string
	Type        string
	Enabled     bool
	State       ServiceState
	ToolCount   int
	Error       string
	Source      string
}

type runtimeService struct {
	config     MCP
	source     string
	state      ServiceState
	client     *McpClient
	toolNames  []string
	toolCount  int
	lastError  string
	stderr     *recentBuffer
	generation uint64
	cancel     context.CancelFunc
}

type Manager struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	services map[string]*runtimeService
	events   chan struct{}
	started  bool
	closed   bool
}

func NewManager() *Manager {
	return &Manager{services: make(map[string]*runtimeService), events: make(chan struct{}, 64)}
}

func (m *Manager) notify() {
	select {
	case m.events <- struct{}{}:
	default:
	}
}

func (m *Manager) Events() <-chan struct{} { return m.events }

func (m *Manager) Start(ctx context.Context) error {
	cfg, sources, err := loadMcpConfigWithSources()
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = true
	m.ctx, m.cancel = context.WithCancel(ctx)
	if err != nil {
		m.mu.Unlock()
		m.notify()
		return fmt.Errorf("load mcp config: %w", err)
	}
	for name, server := range cfg.MCPServers {
		state := StateStarting
		if !server.IsEnabled() {
			state = StateDisabled
		}
		m.services[name] = &runtimeService{config: server, source: sources[name], state: state, stderr: &recentBuffer{limit: 8192}}
	}
	m.mu.Unlock()
	m.notify()
	for name, server := range cfg.MCPServers {
		if server.IsEnabled() {
			go m.startService(name)
		}
	}
	return nil
}

func (m *Manager) startService(name string) {
	m.mu.Lock()
	service := m.services[name]
	if service == nil || !service.config.IsEnabled() || m.closed {
		m.mu.Unlock()
		return
	}
	service.generation++
	generation := service.generation
	service.state = StateStarting
	service.lastError = ""
	service.stderr = &recentBuffer{limit: 8192}
	serviceCtx, cancel := context.WithCancel(m.ctx)
	service.cancel = cancel
	cfg, stderr, ctx := service.config, service.stderr, serviceCtx
	m.mu.Unlock()
	m.notify()

	client, serverTools, err := runMcpByType(ctx, cfg, stderr)
	if err == nil {
		var names []string
		names, err = RegisterMcpTools(name, client, serverTools)
		if err == nil {
			m.mu.Lock()
			service = m.services[name]
			if service == nil || service.generation != generation || !service.config.IsEnabled() || m.closed {
				m.mu.Unlock()
				RemoveMcpTools(names)
				_ = client.Close()
				return
			}
			service.client = client
			service.cancel = nil
			service.toolNames = names
			service.toolCount = len(serverTools)
			service.state = StateReady
			m.mu.Unlock()
			m.notify()
			return
		}
	}
	if client != nil {
		_ = client.Close()
	}
	message := err.Error()
	if captured := stderr.String(); captured != "" {
		message += "\n" + captured
	}
	m.mu.Lock()
	service = m.services[name]
	if service != nil && service.generation == generation {
		service.cancel = nil
		service.state = StateError
		service.lastError = message
	}
	m.mu.Unlock()
	m.notify()
}

func (m *Manager) Snapshot() []ServiceStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]ServiceStatus, 0, len(m.services))
	for name, service := range m.services {
		errText := service.lastError
		if captured := service.stderr.String(); captured != "" && !strings.Contains(errText, captured) {
			if errText != "" {
				errText += "\n"
			}
			errText += captured
		}
		result = append(result, ServiceStatus{
			Name: name, Description: serviceDescription(name, service.config, service.toolCount), Type: service.config.Type,
			Enabled: service.config.IsEnabled(), State: service.state, ToolCount: service.toolCount, Error: errText, Source: service.source,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func serviceDescription(name string, cfg MCP, toolCount int) string {
	if description := strings.TrimSpace(cfg.Description); description != "" {
		return description
	}
	target := cfg.URL
	if target == "" {
		target = strings.TrimSpace(strings.Join(append([]string{cfg.Command}, cfg.Args...), " "))
	}
	parts := []string{cfg.Type + " MCP"}
	if target != "" {
		parts = append(parts, target)
	} else if name != "" {
		parts = append(parts, name)
	}
	if toolCount > 0 {
		parts = append(parts, fmt.Sprintf("%d tools", toolCount))
	}
	return strings.Join(parts, " · ")
}

func (m *Manager) ErrorSummary() string {
	statuses := m.Snapshot()
	var failed []string
	for _, status := range statuses {
		if status.State == StateError {
			failed = append(failed, status.Name)
		}
	}
	if len(failed) == 0 {
		return ""
	}
	return fmt.Sprintf("MCP: %d failed (%s)", len(failed), strings.Join(failed, ", "))
}

func boolPointer(value bool) *bool { return &value }

func persistEnabled(path, name string, enabled bool) error {
	cfg, err := loadMcpConfigFile(path)
	if err != nil {
		return err
	}
	server, ok := cfg.MCPServers[name]
	if !ok {
		return fmt.Errorf("MCP server %q is not present in %q", name, path)
	}
	server.Enabled = boolPointer(enabled)
	cfg.MCPServers[name] = server
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode mcp config %q: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write mcp config %q: %w", path, err)
	}
	return nil
}

func (m *Manager) SetEnabled(name string, enabled bool) error {
	m.mu.Lock()
	service := m.services[name]
	if service == nil {
		m.mu.Unlock()
		return fmt.Errorf("MCP server %q not found", name)
	}
	if service.config.IsEnabled() == enabled {
		m.mu.Unlock()
		return nil
	}
	source := service.source
	m.mu.Unlock()
	if err := persistEnabled(source, name, enabled); err != nil {
		return err
	}

	m.mu.Lock()
	service = m.services[name]
	service.config.Enabled = boolPointer(enabled)
	service.generation++
	client, names, cancel := service.client, append([]string(nil), service.toolNames...), service.cancel
	service.client = nil
	service.cancel = nil
	service.toolNames = nil
	service.toolCount = 0
	service.lastError = ""
	if enabled {
		service.state = StateStarting
	} else {
		service.state = StateDisabled
	}
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	RemoveMcpTools(names)
	_ = client.Close()
	m.notify()
	if enabled {
		go m.startService(name)
	}
	return nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	if m.cancel != nil {
		m.cancel()
	}
	var clients []*McpClient
	var names []string
	for _, service := range m.services {
		service.generation++
		if service.cancel != nil {
			service.cancel()
			service.cancel = nil
		}
		clients = append(clients, service.client)
		names = append(names, service.toolNames...)
		service.client = nil
		service.toolNames = nil
		if service.state != StateDisabled {
			service.state = StateStopped
		}
	}
	m.mu.Unlock()
	RemoveMcpTools(names)
	var errs []error
	for _, client := range clients {
		if err := client.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	m.notify()
	return errors.Join(errs...)
}

var defaultManager = NewManager()

func RunMcp(ctx context.Context) ([]*McpClient, error) {
	return nil, defaultManager.Start(ctx)
}
func DefaultManager() *Manager { return defaultManager }
func ClearMcpState() {
	_ = defaultManager.Close()
	clearAllMcpTools()
	defaultManager = NewManager()
}

// McpToolEntry 描述一个暴露给模型的 MCP 工具。
type McpToolEntry struct {
	Client      *McpClient
	Server      string
	ToolName    string
	Description string
	InputSchema any
}

var (
	McpToolRegistry = map[string]McpToolEntry{}
	mcpStateMu      sync.RWMutex
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
	return strings.Trim(invalidNameRun.ReplaceAllString(value, "_"), "_-")
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
		seen[exposed] = struct{}{}
		exposedName := exposed
		pending = append(pending, pendingTool{
			name:  exposed,
			entry: McpToolEntry{Client: client, Server: serverName, ToolName: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema},
			fn: tools.ToolFunc{Name: exposed, Function: func(ctx context.Context, args []any) (string, error) {
				_, result, err := SelectAndCallMcp(ctx, exposedName, mcpArgsToJSON(args))
				return result, err
			}},
			schema: tools.ToolReg{Type: "function", Function: tools.ToolFunction{Name: exposed, Description: tool.Description, Parameters: tool.InputSchema}},
		})
	}
	names := make([]string, 0, len(pending))
	functions := make(map[string]tools.ToolFunc, len(pending))
	schemas := make([]tools.ToolReg, 0, len(pending))
	for _, item := range pending {
		McpToolRegistry[item.name] = item.entry
		names = append(names, item.name)
		functions[item.name] = item.fn
		schemas = append(schemas, item.schema)
	}
	tools.RegisterMcpToolSet(functions, schemas)
	return names, nil
}

func RemoveMcpTools(names []string) {
	if len(names) == 0 {
		return
	}
	mcpStateMu.Lock()
	for _, name := range names {
		delete(McpToolRegistry, name)
	}
	mcpStateMu.Unlock()
	tools.RemoveMcpTools(names)
}

func clearAllMcpTools() {
	mcpStateMu.Lock()
	McpToolRegistry = map[string]McpToolEntry{}
	mcpStateMu.Unlock()
	tools.ClearMcpTools()
}

func RegisteredMcpToolNames() []string {
	mcpStateMu.RLock()
	defer mcpStateMu.RUnlock()
	names := make([]string, 0, len(McpToolRegistry))
	for name := range McpToolRegistry {
		names = append(names, name)
	}
	return names
}

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

func SelectAndCallMcp(ctx context.Context, exposedName, argumentsJSON string) (bool, string, error) {
	mcpStateMu.RLock()
	entry, ok := McpToolRegistry[exposedName]
	mcpStateMu.RUnlock()
	if !ok {
		return false, "", nil
	}
	var args map[string]any
	if argumentsJSON != "" {
		if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
			return true, "", fmt.Errorf("parse mcp arguments for %q: %w", exposedName, err)
		}
	}
	res, err := entry.Client.Session.CallTool(ctx, &mcpsdk.CallToolParams{Name: entry.ToolName, Arguments: args})
	if err != nil {
		return true, "", fmt.Errorf("call mcp %s/%s: %w", entry.Server, entry.ToolName, err)
	}
	return true, mcpResultToString(res), nil
}

func mcpResultToString(res *mcpsdk.CallToolResult) string {
	var sb strings.Builder
	for _, content := range res.Content {
		if text, ok := content.(*mcpsdk.TextContent); ok {
			sb.WriteString(text.Text)
			continue
		}
		data, _ := json.Marshal(content)
		sb.Write(data)
	}
	if res.IsError {
		return "MCP tool error: " + sb.String()
	}
	return sb.String()
}
