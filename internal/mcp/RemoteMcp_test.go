package mcp

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"orbit/internal/tools"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestLoadMcpconfigParsesNamedServers(t *testing.T) {
	tempDir := t.TempDir()
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldCwd) })

	content := `{
		"mcpServers": {
			"time": {
				"command": "npx",
				"args": ["-y", "@guanxiong/mcp-server-time"],
				"env": {"TZ": "Asia/Shanghai"}
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(tempDir, ".mcp.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := LoadMcpconfig()
	if err != nil {
		t.Fatalf("LoadMcpconfig() error = %v", err)
	}

	server, ok := config.MCPServers["time"]
	if !ok {
		t.Fatalf("MCPServers does not contain time: %#v", config.MCPServers)
	}
	if server.Command != "npx" {
		t.Fatalf("Command = %q, want %q", server.Command, "npx")
	}
	if !reflect.DeepEqual(server.Args, []string{"-y", "@guanxiong/mcp-server-time"}) {
		t.Fatalf("Args = %#v", server.Args)
	}
	if server.Env["TZ"] != "Asia/Shanghai" {
		t.Fatalf("Env[TZ] = %q", server.Env["TZ"])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestHeaderInjectingTransportAllowsSameOrigin(t *testing.T) {
	origin, err := newHTTPClientWithHeaders("https://example.com/mcp", map[string]string{"Authorization": "Bearer secret"})
	if err != nil {
		t.Fatal(err)
	}
	transport := origin.Transport.(*headerInjectingTransport)
	transport.base = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q", got)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})

	req, _ := http.NewRequest(http.MethodPost, "https://example.com/next", nil)
	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
}

func TestHeaderInjectingTransportRejectsCrossOrigin(t *testing.T) {
	client, err := newHTTPClientWithHeaders("https://example.com/mcp", map[string]string{"Authorization": "Bearer secret"})
	if err != nil {
		t.Fatal(err)
	}
	transport := client.Transport.(*headerInjectingTransport)
	called := false
	transport.base = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return nil, nil
	})

	req, _ := http.NewRequest(http.MethodPost, "https://evil.example/endpoint", nil)
	if _, err := transport.RoundTrip(req); err == nil || !strings.Contains(err.Error(), "cross-origin") {
		t.Fatalf("RoundTrip() error = %v, want cross-origin rejection", err)
	}
	if called {
		t.Fatal("base transport was called for cross-origin request")
	}
}

func TestHTTPClientRejectsCrossOriginRedirect(t *testing.T) {
	client, err := newHTTPClientWithHeaders("https://example.com/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://evil.example/redirected", nil)
	if err := client.CheckRedirect(req, nil); err == nil || !strings.Contains(err.Error(), "cross-origin") {
		t.Fatalf("CheckRedirect() error = %v, want cross-origin rejection", err)
	}
}

func TestExposedToolNameIsStableValidAndCollisionResistant(t *testing.T) {
	first := ExposedToolName("a__b", "c")
	second := ExposedToolName("a", "b__c")
	if first == second {
		t.Fatalf("ambiguous tuples produced same name %q", first)
	}
	if first != ExposedToolName("a__b", "c") {
		t.Fatal("ExposedToolName is not stable")
	}
	valid := regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	for _, name := range []string{first, second, ExposedToolName(strings.Repeat("服", 100), strings.Repeat("tool!", 100))} {
		if !valid.MatchString(name) {
			t.Fatalf("invalid exposed tool name %q", name)
		}
	}
}

func TestRegisterMcpToolsIsAtomicOnConflict(t *testing.T) {
	ClearMcpState()
	t.Cleanup(ClearMcpState)
	tool := &mcpsdk.Tool{Name: "duplicate", Description: "test", InputSchema: map[string]any{"type": "object"}}
	if _, err := RegisterMcpTools("server", &McpClient{}, []*mcpsdk.Tool{tool, tool}); err == nil {
		t.Fatal("RegisterMcpTools() error = nil, want duplicate conflict")
	}
	if len(RegisteredMcpToolNames()) != 0 || len(tools.RegMcpToolFuncs) != 0 || len(tools.RegisteredMcpTools()) != 0 {
		t.Fatal("failed registration partially committed MCP state")
	}
}

func TestClearMcpStateClearsAllRegistries(t *testing.T) {
	ClearMcpState()
	tool := &mcpsdk.Tool{Name: "echo", Description: "test", InputSchema: map[string]any{"type": "object"}}
	names, err := RegisterMcpTools("server", &McpClient{}, []*mcpsdk.Tool{tool})
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || len(RegisteredMcpToolNames()) != 1 || len(tools.RegMcpToolFuncs) != 1 || len(tools.RegisteredMcpTools()) != 1 {
		t.Fatal("MCP tool was not registered in all registries")
	}

	ClearMcpState()
	if len(RegisteredMcpToolNames()) != 0 || len(tools.RegMcpToolFuncs) != 0 || len(tools.RegisteredMcpTools()) != 0 {
		t.Fatal("ClearMcpState did not clear registry, schema, and execution functions")
	}
}

func TestRegisteredMcpToolFunctionReceivesCallContext(t *testing.T) {
	ClearMcpState()
	t.Cleanup(ClearMcpState)
	name := ExposedToolName("server", "echo")
	McpToolRegistry[name] = McpToolEntry{}
	tools.RegMcpToolFuncs[name] = tools.ToolFunc{
		Name: name,
		Function: func(ctx context.Context, _ []any) (string, error) {
			return "", ctx.Err()
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tools.RegMcpToolFuncs[name].Function(ctx, nil); err != context.Canceled {
		t.Fatalf("Function() error = %v, want context.Canceled", err)
	}
}
