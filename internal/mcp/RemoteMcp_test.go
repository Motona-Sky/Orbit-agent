package mcp

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"looporbit/internal/utils"
)

func TestLoadMcpconfigParsesNamedServers(t *testing.T) {
	tempDir := t.TempDir()
	oldCwd := utils.Cwd
	utils.Cwd = tempDir
	t.Cleanup(func() { utils.Cwd = oldCwd })

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
