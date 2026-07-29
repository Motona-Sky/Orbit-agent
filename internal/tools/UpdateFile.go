package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// UpdateFile 创建/覆写文件，或将文件中唯一匹配 old_content 的片段替换为 new_content。
// old_content 为空时表示整文件写入；非空时要求文件已存在，且 old_content 恰好出现一次。
func UpdateFile(path string, old_content string, new_content string) error {
	if isBinaryContent([]byte(old_content)) {
		return fmt.Errorf("old_content must be valid UTF-8 text without NUL bytes")
	}
	if isBinaryContent([]byte(new_content)) {
		return fmt.Errorf("new_content must be valid UTF-8 text without NUL bytes")
	}

	// 整文件写入模式：创建新文件或全量覆写
	if old_content == "" {
		return writeFileContent(path, []byte(new_content))
	}

	// 替换模式：直接读取完整文件，避免只读工具的展示行数限制影响更新。
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", path)
		}
		return err
	}

	// 拒绝二进制与 UTF-16 文件，避免按字节替换破坏内容
	if isBinaryContent(content) {
		return fmt.Errorf("cannot update file containing NUL bytes, invalid UTF-8, or a UTF-16 BOM: %s", path)
	}

	text := string(content)
	match := strings.Index(text, old_content)
	if match < 0 {
		return fmt.Errorf("old_content not found in %s", path)
	}
	if strings.Index(text[match+1:], old_content) >= 0 {
		return fmt.Errorf("old_content found multiple times in %s, provide more surrounding context to make it unique", path)
	}

	newText := text[:match] + new_content + text[match+len(old_content):]
	return writeFileContent(path, []byte(newText))
}

func writeFileContent(path string, content []byte) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	targetPath := path
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		targetPath, err = filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	mode := os.FileMode(0644)
	targetExists := false
	if info, err = os.Stat(targetPath); err == nil {
		mode = info.Mode().Perm()
		targetExists = true
	} else if !os.IsNotExist(err) {
		return err
	}

	targetDir := filepath.Dir(targetPath)
	reservation, err := os.CreateTemp(targetDir, "."+filepath.Base(targetPath)+"-*")
	if err != nil {
		return err
	}
	tempPath := reservation.Name()
	if err := reservation.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}
	if err := os.Remove(tempPath); err != nil {
		return err
	}
	temp, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer func() {
		temp.Close()
		os.Remove(tempPath)
	}()

	if _, err := temp.Write(content); err != nil {
		return err
	}
	if targetExists {
		if err := temp.Chmod(mode); err != nil {
			return err
		}
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, targetPath)
}

// isBinaryContent 探测内容是否无法安全按 UTF-8 文本替换。
func isBinaryContent(content []byte) bool {
	if len(content) >= 2 &&
		((content[0] == 0xff && content[1] == 0xfe) || (content[0] == 0xfe && content[1] == 0xff)) {
		return true
	}
	return bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content)
}

// Json传入，运行入口
func CallUpdateFileFunc(jsonstr []any) (string, error) {
	if len(jsonstr) == 0 {
		return "", fmt.Errorf("tool calls is empty UpdateFileFunc")
	}
	function, ok := jsonstr[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("json is not map[string]any UpdateFileFunc")
	}
	arguments, ok := function["arguments"].(string)
	if !ok {
		return "", fmt.Errorf("arguments is not string UpdateFileFunc")
	}
	var args struct {
		Path       string `json:"path"`
		OldContent string `json:"old_content"`
		NewContent string `json:"new_content"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", err
	}
	if err := UpdateFile(args.Path, args.OldContent, args.NewContent); err != nil {
		return "", err
	}
	return "File updated successfully: " + args.Path, nil
}

var UpdateFileDescription = "Create a new file, overwrite an existing file, or replace a unique text fragment within a file.If `old_content` is empty, `new_content` is written as the entire file content.Otherwise, `old_content` must match exactly one location in the file and will be replaced with `new_content`. If no match is found or multiple matches are found, the operation fails and the file remains unchanged.Files containing NUL bytes, invalid UTF-8, or a UTF-16 BOM are not supported."

func GetUpdateFileParameters() ToolParameters {
	path := ToolPropertiesArray{
		Type:        "string",
		Description: "Update File path. Do not enter a folder path",
	}
	oldContent := ToolPropertiesArray{
		Type:        "string",
		Description: "Text to be replaced. Pass an empty string to write the entire file instead.",
	}
	newContent := ToolPropertiesArray{
		Type:        "string",
		Description: "Replacement text, or the full file content when old_content is empty.",
	}
	ToolProperties := map[string]ToolPropertiesArray{
		"path":        path,
		"old_content": oldContent,
		"new_content": newContent,
	}
	return ToolParameters{
		Type:                 "object",
		Properties:           ToolProperties,
		Required:             []string{"path", "old_content", "new_content"},
		AdditionalProperties: false,
	}
}

// 注册工具
func init() {
	UpdateFileparametersJSON := GetUpdateFileParameters()
	RegTools([]ToolReg{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "update_file",
				Description: UpdateFileDescription,
				Parameters:  UpdateFileparametersJSON,
			},
		},
	})
	RegToolFuncs["update_file"] = ToolFunc{
		Name:     "update_file",
		Function: CallUpdateFileFunc,
	}
}
