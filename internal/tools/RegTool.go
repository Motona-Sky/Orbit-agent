package tools

type ToolReg struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

type ToolFunc struct {
	Name     string
	Function func(jsonstr []any) (string, error)
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

var RegToolFuncs = make(map[string]ToolFunc)

var registeredTools []ToolReg

func RegTools(tools []ToolReg) []ToolReg {
	registeredTools = append(registeredTools, tools...)
	return registeredTools
}

// 获取所有工具

func GetAllTool(provider string) []ToolReg {
	switch provider {
	case "openai:completions":
		return registeredTools
	case "openai:responses":
		return nil
	case "anthropic:messages":
		return nil
	default:
		return nil
	}
}
