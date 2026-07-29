package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxTreeDepth limits recursion depth to guard against symlink loops.
const maxTreeDepth = 3

// skipDirs are directory names that are excluded from tree listings.
var skipDirs = map[string]bool{
	".svn":         true,
	".git":         true,
	"node_modules": true,
	".venv":        true,
}

func Ls(Path string, recursive bool, depth int) ([]string, error) {
	if Path == "" {
		Path = "."
	}

	if depth <= 0 {
		if depth == 0 {
			depth = maxTreeDepth
		}
		depth = maxTreeDepth
	}

	// 判断路径是否存在
	info, err := os.Stat(Path)
	if err != nil {
		return nil, err
	}

	// 如果 Path 是文件，直接返回文件路径
	if !info.IsDir() {
		return []string{Path}, nil
	}

	switch recursive {
	case true:
		return lsTree(Path, depth)

	case false:
		files, err := os.ReadDir(Path)
		if err != nil {
			return nil, err
		}

		var names []string

		for _, file := range files {
			name := file.Name()
			if file.IsDir() {
				name += "/"
			}
			names = append(names, name)
		}

		return names, nil

	default:
		return Ls(Path, false, depth)
	}
}

// lsTree recursively lists the directory at root in a tree-like format
// similar to the Linux `tree` command. Returned paths are relative to root.
func lsTree(path string, depth int) ([]string, error) {
	return lsTreeRel(path, "", depth)
}

// lsTreeRel is the recursive worker. path is the filesystem location to read,
// prefix is the path relative to the original root used for display output.
func lsTreeRel(path, prefix string, depth int) ([]string, error) {
	var result []string

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if depth <= 0 {
			continue
		}
		name := entry.Name()

		// 排除版本控制、依赖等目录，避免递归进入
		if entry.IsDir() && skipDirs[name] {
			continue
		}

		rel := name
		if prefix != "" {
			rel = filepath.Join(prefix, name)
		}

		// 保存当前文件或目录的相对路径，目录加上 "/" 后缀以便区分
		display := rel
		if entry.IsDir() {
			display += "/"
		}
		result = append(result, display)

		// 如果是目录，继续递归
		if entry.IsDir() {
			childPath := filepath.Join(path, name)
			childResult, err := lsTreeRel(childPath, rel, depth-1)
			if err != nil {
				return nil, err
			}

			result = append(result, childResult...)
		}
	}

	return result, nil
}
func CallLsFunc(jsonstr []any) (string, error) {
	if len(jsonstr) == 0 {
		return "", fmt.Errorf("jsonstr is empty")
	}
	function, ok := jsonstr[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("json is not map[string]any LsFunc")
	}
	arguments, ok := function["arguments"].(string)
	if !ok {
		return "", fmt.Errorf("arguments is not string LsFunc")
	}
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
		Depth     int    `json:"depth"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", err
	}
	files, err := Ls(args.Path, args.Recursive, args.Depth)
	if err != nil {
		return "", err
	}
	return strings.Join(files, "\n"), nil
}

func GetLsParameters() ToolParameters {

	path := ToolPropertiesArray{
		Type:        "string",
		Description: "Directory path to list.",
	}
	recursive := ToolPropertiesArray{
		Type:        "boolean",
		Description: "Enable or disable recursive directory listing.",
	}
	depth := ToolPropertiesArray{
		Type:        "integer",
		Description: "Recursion depth. Default is 3, false not used.",
	}
	ToolProperties := map[string]ToolPropertiesArray{
		"path":      path,
		"recursive": recursive,
		"depth":     depth,
	}
	return ToolParameters{
		Type:                 "object",
		Properties:           ToolProperties,
		Required:             []string{"path", "recursive"},
		AdditionalProperties: false,
	}

}

func init() {
	LsparametersJSON := GetLsParameters()
	RegTools([]ToolReg{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "ls",
				Description: "List the entries in the specified directory. Set `recursive=true` to recursively list all nested files in depth-first order, skipping the `.git` and `node_modules` directories.",
				Parameters:  LsparametersJSON,
			},
		},
	})
	RegToolFuncs["ls"] = ToolFunc{
		Name:     "ls",
		Function: CallLsFunc,
	}
}
