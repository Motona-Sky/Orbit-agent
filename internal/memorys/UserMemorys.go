package memorys

import (
	"context"
	"encoding/json"
	"fmt"
	"orbit/internal/tools"
	"orbit/internal/utils"
	"os"
	"path/filepath"
)

type UserMemory struct {
	Num         int            `json:"num"`
	UserMemorys map[int]string `json:"user_memorys"`
}

type UserMemorys []UserMemory

var userMemPath = filepath.Join(
	utils.ConfigFolderPath,
	"memorys.json",
)

func AddUserMemorys(userMemory string) error {
	var u UserMemorys
	// 读取已有文件
	mem, err := os.ReadFile(userMemPath)
	if err != nil && !os.IsNotExist(err) {
		os.Remove(userMemPath)
		os.Create(userMemPath)
	}

	// 文件存在且不为空时，解析已有 JSON
	if len(mem) > 0 {
		_ = json.Unmarshal(mem, &u)
	}

	// 找出下一个编号
	nextNum := 1

	for _, item := range u {
		if item.Num >= nextNum {
			nextNum = item.Num + 1
		}
	}
	if nextNum >= 11 {
		return fmt.Errorf("user memory limit reached (maximum: 10)")
	}
	// 添加一条新记忆
	u = append(u, UserMemory{
		Num: nextNum,
		UserMemorys: map[int]string{
			nextNum: userMemory,
		},
	})

	// 转换成 JSON
	data, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件；文件不存在时会自动创建
	if err := os.WriteFile(userMemPath, data, 0600); err != nil {
		return err
	}
	return nil
}

func GetUserMemorys() (UserMemorys, error) {
	var u UserMemorys
	mem, err := os.ReadFile(userMemPath)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(mem, &u)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUserMemorysPrompt() string {
	var prompt string
	userMemorys, err := GetUserMemorys()
	if err != nil {
		return ""
	}
	for num, item := range userMemorys {
		prompt += fmt.Sprintf("%d.  %s\n", num+1, item.UserMemorys[item.Num])
	}
	return prompt
}

func DelUserMemorys(num int) error {
	userMemorys, err := GetUserMemorys()
	if err != nil {
		return err
	}

	// 从切片中移除匹配的元素，并重新编号保持连续
	var filtered UserMemorys
	newNum := 1
	for _, item := range userMemorys {
		if item.Num != num {
			oldNum := item.Num
			item.Num = newNum
			item.UserMemorys = map[int]string{newNum: item.UserMemorys[oldNum]}
			filtered = append(filtered, item)
			newNum++
		}
	}

	// 写回文件
	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(userMemPath, data, 0600)
}
func UpdateUserMemory(addumem string, delumem int) string {
	if delumem != 0 {
		DelUserMemorys(delumem)
	}
	if addumem != "" {
		err := AddUserMemorys(addumem)
		if err != nil {
			if err.Error() == "user memory limit reached (maximum: 10)" {
				return "user memory limit reached (maximum: 10)"
			}
		}
	}
	Usermem := GetUserMemorysPrompt()
	return fmt.Sprintf("Update Usermemory success\nUsermemory:%s", Usermem)
}

func CallUpdateUserMemoryFunc(_ context.Context, jsonstr []any) (string, error) {
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
		Addumem string `json:"addumem"`
		Delumem int    `json:"delumem"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", err
	}
	toolreturn := UpdateUserMemory(args.Addumem, args.Delumem)
	return toolreturn, nil
}

func GetLsParameters() tools.ToolParameters {

	Addumem := tools.ToolPropertiesArray{
		Type:        "string",
		Description: "Add a single Usermemory",
	}
	Delumem := tools.ToolPropertiesArray{
		Type:        "integer",
		Description: "Delete a single Usermemory",
	}
	ToolProperties := map[string]tools.ToolPropertiesArray{
		"Addumem": Addumem,
		"Delumem": Delumem,
	}
	return tools.ToolParameters{
		Type:                 "object",
		Properties:           ToolProperties,
		Required:             []string{},
		AdditionalProperties: false,
	}

}

func init() {
	LsparametersJSON := GetLsParameters()
	tools.RegTools([]tools.ToolReg{
		{
			Type: "function",
			Function: tools.ToolFunction{
				Name:        "UpdateUserMemory",
				Description: "Updates user memories maximum:10,If both are provided, delete first, then add.",
				Parameters:  LsparametersJSON,
			},
		},
	})
	tools.RegToolFuncs["UpdateUserMemory"] = tools.ToolFunc{
		Name:     "UpdateUserMemory",
		Function: CallUpdateUserMemoryFunc,
	}
}
