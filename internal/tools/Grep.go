package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// type: filename,content
// path: file path or directory path
func Grep(Type string, path string, content string) ([]string, error) {
	if content == "" {
		return nil, fmt.Errorf("content must not be empty")
	}

	var result []string
	switch Type {
	case "filename":
		if path == "" {
			path = "."
		}
		allfile, err := lsTreeRel(path, "", 32767)
		if err != nil {
			return nil, err
		}
		for _, file := range allfile {
			if strings.Contains(file, content) {
				result = append(result, file)
			}
		}
		return result, nil
		// 搜索文件
	case "content":
		return grepContent(path, content)
	default:
		return nil, nil
	}
}

type grepTarget struct {
	path    string
	display string
}

func grepContent(path, pattern string) ([]string, error) {
	if path == "" {
		path = "."
	}
	expression, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	targets, err := grepTargets(path)
	if err != nil {
		return nil, err
	}

	var result []string
	var warnings []string
	for _, target := range targets {
		file, err := os.Open(target.path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("[Warning] %s: %v", target.display, err))
			continue
		}

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		var matches []string
		lineNumber := 0
		binary := false
		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()
			if strings.IndexByte(line, 0) >= 0 {
				binary = true
				break
			}
			if expression.MatchString(line) {
				matches = append(matches, fmt.Sprintf("%s:%d:%s", target.display, lineNumber, line))
			}
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if binary {
			continue
		}
		if scanErr != nil {
			warnings = append(warnings, fmt.Sprintf("[Warning] %s: %v", target.display, scanErr))
			continue
		}
		if closeErr != nil {
			warnings = append(warnings, fmt.Sprintf("[Warning] %s: %v", target.display, closeErr))
			continue
		}
		result = append(result, matches...)
	}
	return append(result, warnings...), nil
}

func grepTargets(path string) ([]grepTarget, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []grepTarget{{path: path, display: filepath.Base(path)}}, nil
	}

	entries, err := lsTreeRel(path, "", 32767)
	if err != nil {
		return nil, err
	}
	targets := make([]grepTarget, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry, "/") {
			continue
		}
		targets = append(targets, grepTarget{
			path:    filepath.Join(path, entry),
			display: entry,
		})
	}
	return targets, nil
}

func CallGrepFunc(_ context.Context, jsonstr []any) (string, error) {
	if len(jsonstr) == 0 {
		return "", fmt.Errorf("jsonstr is empty")
	}
	function, ok := jsonstr[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("json is not map[string]any GrepFunc")
	}
	arguments, ok := function["arguments"].(string)
	if !ok {
		return "", fmt.Errorf("arguments is not string GrepFunc")
	}
	var args struct {
		Type    string `json:"type"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", err
	}
	files, err := Grep(args.Type, args.Path, args.Content)
	if err != nil {
		return "", err
	}
	return strings.Join(files, "\n"), nil
}

func GetGrepParameters() ToolParameters {

	Type := ToolPropertiesArray{
		Type:        "string",
		Description: "Search type. filename or content.",
	}
	path := ToolPropertiesArray{
		Type:        "string",
		Description: "Directory path or file path.",
	}
	content := ToolPropertiesArray{
		Type:        "string",
		Description: "Search content.if type is filename,search file name.if type is content,search file content.",
	}
	ToolProperties := map[string]ToolPropertiesArray{
		"type":    Type,
		"path":    path,
		"content": content,
	}
	return ToolParameters{
		Type:                 "object",
		Properties:           ToolProperties,
		Required:             []string{"type", "content"},
		AdditionalProperties: false,
	}

}

func init() {
	GrepparametersJSON := GetGrepParameters()
	RegTools([]ToolReg{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "Grep",
				Description: "Search by file name or file content to locate the file containing a specified field, keyword, or code.",
				Parameters:  GrepparametersJSON,
			},
		},
	})
	RegToolFuncs["Grep"] = ToolFunc{
		Name:     "Grep",
		Function: CallGrepFunc,
	}
}
