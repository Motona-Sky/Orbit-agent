package memorys

import (
	"encoding/json"
	"fmt"
	"looporbit/internal/utils"
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
		return "no user memorys"
	}
	for _, item := range userMemorys {
		prompt += fmt.Sprintf("- %s\n", item.UserMemorys[item.Num])
	}
	return prompt
}

func DelUserMemorys(num int) error {
	userMemorys, err := GetUserMemorys()
	if err != nil {
		return err
	}
	for _, item := range userMemorys {
		if item.Num == num {
			delete(item.UserMemorys, item.Num)
		}
	}
	return nil
}
