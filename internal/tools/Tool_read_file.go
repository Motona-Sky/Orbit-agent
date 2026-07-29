package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"looporbit/internal/utils"
	"os"
	"strings"
)

// FileType 判断文件是否为可读取的文本文件，并返回文本内容或二进制文件头。内部函数
func FileType(filepath string) (string, bool, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	peek := make([]byte, 8*1024)
	n, err := io.ReadFull(f, peek)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", false, err
	}
	peek = peek[:n]

	isText := false

	// 带 BOM 的 UTF-16 是文本；其内容中的 NUL 字节不能作为二进制依据。
	if len(peek) >= 2 && ((peek[0] == 0xff && peek[1] == 0xfe) ||
		(peek[0] == 0xfe && peek[1] == 0xff)) {
		isText = true
	}

	// 识别无 BOM 的 UTF-16：NUL 字节应主要稳定出现在同一奇偶位置。
	pairs := len(peek) / 2
	if !isText && pairs > 0 {
		var zeroEven, zeroOdd int
		for i := 0; i < pairs*2; i += 2 {
			if peek[i] == 0 {
				zeroEven++
			}
			if peek[i+1] == 0 {
				zeroOdd++
			}
		}
		if (zeroEven*2 >= pairs && zeroOdd == 0) ||
			(zeroOdd*2 >= pairs && zeroEven == 0) {
			isText = true
		}
	}

	if !isText {
		isText = bytes.IndexByte(peek, 0) < 0
	}
	if !isText {
		if len(peek) > 8 {
			peek = peek[:8]
		}
		return string(peek), false, nil
	}

	rest, err := io.ReadAll(f)
	if err != nil {
		return "", false, err
	}
	Text := string(append(peek, rest...))
	if len(Text) == 0 {
		return Text, false, fmt.Errorf("The file is empty")
	}
	return Text, true, nil
}

// ReadText 从零基行号开始截取文本，并返回原文总行数。
func ReadText(Text string, startline int, limitline int) (string, int, error) {
	if Text == "" {
		return "", 0, fmt.Errorf("startline %d exceeds total lines 0", startline)
	}

	lines := strings.SplitAfter(Text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	totalLines := len(lines)

	if startline < 0 {
		startline = 0
	}
	if startline >= totalLines {
		return "", totalLines, fmt.Errorf("startline %d exceeds total lines %d", startline, totalLines)
	}
	if limitline <= 0 {
		limitline = 1000
	}

	endline := totalLines
	if limitline < totalLines-startline {
		endline = startline + limitline
	}
	return strings.Join(lines[startline:endline], ""), totalLines, nil
}

// 默认返回前1000行
func ReadFile(filepath string, startline int, limitline int) (string, error) {
	Text, Textbool, err := FileType(filepath)
	if err != nil {
		return "", err
	}
	if !Textbool {
		return fmt.Sprintf("The file header is %s", Text), nil
	} else {
		Texts, _, err := ReadText(Text, startline, limitline)
		if err != nil {
			return "", err
		}
		return Texts, nil //正常返回逻辑

	}

}

// Json传入，运行入口
func CallReadFIleFunc(jsonstr []any) (string, error) {
	if len(jsonstr) == 0 {
		return "", fmt.Errorf("tool calls is empty ReadFileFunc")
	}
	function, ok := jsonstr[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("json is not map[string]any ReadFileFunc")
	}
	arguments, ok := function["arguments"].(string)
	if !ok {
		return "", fmt.Errorf("arguments is not string ReadFileFunc")
	}
	var args struct {
		Filepath  string `json:"filepath"`
		Startline int    `json:"startline"`
		Limitline int    `json:"limitline"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", err
	}
	return ReadFile(args.Filepath, args.Startline, args.Limitline)
}

//	var ReadFileparametersJSON = map[string]any{
//		"type": "object",
//		"properties": map[string]any{
//			"filepath": map[string]any{
//				"type":        "string",
//				"description": "File path. Do not enter a folder path.",
//			},
//			"startline": map[string]any{
//				"type":        "integer",
//				"description": "0-based line offset to start reading from (default 0)",
//				"minimum":     0,
//			},
//			"limitline": map[string]any{
//				"type":        "integer",
//				"description": "Maximum lines to return (default 1000)",
//				"minimum":     0,
//			},
//		},
//		"required":             []string{"filepath"},
//		"additionalProperties": false,
//	}

func GetReadFileParameters() ToolParameters {

	filepath := ToolPropertiesArray{
		Type:        "string",
		Description: "File path. Do not enter a folder path.",
	}
	startline := ToolPropertiesArray{
		Type:        "integer",
		Description: "0-based line offset to start reading from (default 0)",
		Minimum:     utils.IntPtr(0),
	}
	limitline := ToolPropertiesArray{
		Type:        "integer",
		Description: "Maximum lines to return (default 1000)",
		Minimum:     utils.IntPtr(0),
	}
	ToolProperties := map[string]ToolPropertiesArray{
		"filepath":  filepath,
		"startline": startline,
		"limitline": limitline,
	}
	return ToolParameters{
		Type:                 "object",
		Properties:           ToolProperties,
		Required:             []string{"filepath"},
		AdditionalProperties: false,
	}

}

// 注册工具
func init() {
	ReadFileparametersJSON := GetReadFileParameters()
	RegTools([]ToolReg{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "read_file",
				Description: "Read file content",
				Parameters:  ReadFileparametersJSON,
			},
		},
	})
	RegToolFuncs["read_file"] = ToolFunc{
		Name:     "read_file",
		Function: CallReadFIleFunc,
	}
}
