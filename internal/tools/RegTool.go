package tools

import "context"

type ToolReg struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Parameters 用 any 而非强类型 ToolParameters：
	// 本地工具传 ToolParameters 结构体；MCP 工具传 server 返回的原始 JSON Schema（map[string]any）。
	// 两者都能正确序列化为 OpenAI/Anthropic 期望的 JSON Schema 对象。
	Parameters any `json:"parameters"`
}
type ToolFunc struct {
	Name     string
	Function func(context.Context, []any) (string, error)
}
type ToolParameters struct {
	Type                 string                         `json:"type"`
	Properties           map[string]ToolPropertiesArray `json:"properties"`
	Required             []string                       `json:"required"`
	AdditionalProperties bool                           `json:"additionalProperties"`
}
type ToolPropertiesArray struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Minimum     *int   `json:"minimum,omitempty"`
	Maximum     *int   `json:"maximum,omitempty"`
}
type ToolProperties struct {
	Name []ToolPropertiesArray `json:"name"`
}

// RegToolFuncs 存储本地工具的执行函数，由各 Tool_*.go 的 init() 填充。
var RegToolFuncs = make(map[string]ToolFunc)

// RegMcpToolFuncs 存储 MCP 工具的执行函数，由 mcp.RegisterMcpTools 填充。
// 与本地池分开存放，便于 GetEnabledTools 按 McpEnabled 开关整体纳入或排除。
var RegMcpToolFuncs = make(map[string]ToolFunc)

var registeredTools []ToolReg

// registeredMcpTools 存储 MCP 工具的 schema 描述，供 GetAllTool 合并后发给模型。
var registeredMcpTools []ToolReg

func RegTools(tools []ToolReg) []ToolReg {
	registeredTools = append(registeredTools, tools...)
	return registeredTools
}

func RegMcpTools(tools []ToolReg) {
	registeredMcpTools = append(registeredMcpTools, tools...)
}

// ClearMcpTools 清空 MCP 工具注册表（执行函数 + schema）。
// 重新加载 MCP server 前调用，避免旧 server 残留的工具名污染注册表。
func ClearMcpTools() {
	registeredMcpTools = nil
	for name := range RegMcpToolFuncs {
		delete(RegMcpToolFuncs, name)
	}
}

func RegisteredMcpTools() []ToolReg {
	return append([]ToolReg(nil), registeredMcpTools...)
}

func HasLocalTool(name string) bool {
	if _, ok := RegToolFuncs[name]; ok {
		return true
	}
	for _, tool := range registeredTools {
		if tool.Function.Name == name {
			return true
		}
	}
	return false
}

// 获取所有工具

func SpecificTool(toolname string) ToolReg {
	for _, tool := range registeredTools {
		if tool.Function.Name == toolname {
			return tool
		}
	}
	return ToolReg{}
}

func GetSpecificTool(toolname []string) []ToolReg {
	var tools []ToolReg
	for _, toolname := range toolname {
		tool := SpecificTool(toolname)
		tools = append(tools, tool)
	}
	return tools
}

// === 工具开关与分层禁用（包内状态）=============================================
//
// agent 主循环只调 GetAllTool(provider) 和 RunTools(toolCalls)，不感知 MCP 开关
// 和工具禁用细节；过滤逻辑全部封装在这里，保证执行层（RunTools 查的 enabled 集）
// 和声明层（GetAllTool 返回的 schema 列表）按同一规则过滤，避免错位。
//
// 状态由配置 UI 或启动初始化阶段通过 SetMcpEnabled / DisableTool 等设置。

var (
	mcpEnabled    = true // 默认开启 MCP 工具；无 .mcp.json 时 RegMcpToolFuncs 为空，等于无副作用
	disabledTools = make(map[string]bool)
)

// SetMcpEnabled 整体开关 MCP 工具层。
// 设为 false 后，GetAllTool 和 RunTools 都不再纳入 RegMcpToolFuncs/registeredMcpTools。
func SetMcpEnabled(enabled bool) {
	mcpEnabled = enabled
}

// McpEnabled 返回当前 MCP 工具层的开关状态。
func McpEnabled() bool { return mcpEnabled }

// DisableTool 按工具名禁用单个工具（本地或 MCP 都生效）。
// 重复禁用同一工具是幂等的。
func DisableTool(name string) {
	disabledTools[name] = true
}

// EnableTool 解除对单个工具的禁用。未禁用时调用是幂等的。
func EnableTool(name string) {
	delete(disabledTools, name)
}

// SetDisabledTools 用一份列表覆盖当前的禁用集合，旧的禁用项全部清空。
func SetDisabledTools(names []string) {
	disabledTools = make(map[string]bool, len(names))
	for _, n := range names {
		disabledTools[n] = true
	}
}

// IsToolDisabled 判断单个工具是否被禁用。
func IsToolDisabled(name string) bool {
	return disabledTools[name]
}

// getAllToolInternal 是 GetAllTool 和 GetEnabledTools 共用的过滤骨架。
// disabledSet 由调用方预先构造，避免重复计算。
func toolAllowed(name string, disabledSet map[string]bool) bool {
	return !disabledSet[name]
}

// GetAllTool 返回发给模型的工具 schema 列表。
// 内部按 mcpEnabled 和 disabledTools 自动过滤：
//   - mcpEnabled=false 时不并入 MCP 工具
//   - disabledTools 中的工具名（无论本地还是 MCP）一律排除
//
// agent 主循环只需调 GetAllTool(provider)，无需传开关参数。
func GetAllTool(provider string) []ToolReg {
	var result []ToolReg
	switch provider {
	case "openai:completions":
		for _, t := range registeredTools {
			if toolAllowed(t.Function.Name, disabledTools) {
				result = append(result, t)
			}
		}
		if mcpEnabled {
			for _, t := range registeredMcpTools {
				if toolAllowed(t.Function.Name, disabledTools) {
					result = append(result, t)
				}
			}
		}
		return result
	case "openai:responses":
		return nil
	case "anthropic:messages":
		return nil
	default:
		return nil
	}
}

// GetEnabledTools 返回当前可执行的工具集合，供 RunTools 内部查找。
// 与 GetAllTool 用同一套 mcpEnabled / disabledTools 过滤，保证模型看到的工具集 =
// RunTools 实际可执行的集合。
func GetEnabledTools() map[string]ToolFunc {
	result := make(map[string]ToolFunc)
	for name, fn := range RegToolFuncs {
		if toolAllowed(name, disabledTools) {
			result[name] = fn
		}
	}
	if mcpEnabled {
		for name, fn := range RegMcpToolFuncs {
			if toolAllowed(name, disabledTools) {
				result[name] = fn
			}
		}
	}
	return result
}
