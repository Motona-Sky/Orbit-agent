// Package config 提供 Orbit 配置目录路径相关的基础能力。
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigFolderName 是 Orbit 在用户主目录下创建的配置文件夹名称。
const ConfigFolderName = ".orbit"

// 迁移utils.GetSystemVersion
// GetSystemVersion 返回当前运行系统的简短名称。
//
//	func GetSystemVersion() string {
//		switch runtime.GOOS {
//		case "windows":
//			return "windows"
//		case "linux":
//			return "linux"
//		case "darwin":
//			// Go 使用 darwin 表示 macOS，这里按项目可读性转换为 mac。
//			return "mac"
//		default:
//			return runtime.GOOS
//		}
//	}
var ConfigPath string

func init() {
	Config, _ := GetConfigFolderPath()
	ConfigPath = Config["ConfigFolder"]

}

// GetUserHomePath 获取当前用户的主目录路径。//内部函数
func GetUserHomePath() (string, error) {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get user home path: %w", err)
	}
	if homePath == "" {
		return "", fmt.Errorf("user home path is empty")
	}
	return homePath, nil
}

// GetConfigFolderPath 返回 Orbit 配置文件夹的完整路径，但不创建目录。
func GetConfigFolderPath() (map[string]string, error) {
	var CreateFolder map[string]string
	homePath, err := GetUserHomePath()
	if err != nil {
		return nil, err
	}
	ConfigFolder := filepath.Join(homePath, ConfigFolderName)
	ChatHistoryFolder := filepath.Join(ConfigFolder, "sessions")
	CreateFolder = map[string]string{
		"ConfigFolder":      ConfigFolder,
		"ChatHistoryFolder": ChatHistoryFolder,
	}
	return CreateFolder, nil
}

// CreateConfigFolder 在用户主目录下创建 Orbit 配置文件夹，并返回目录路径。
func CreateConfigFolder() (string, error) {
	configPath, err := GetConfigFolderPath()
	if err != nil {
		return "", err
	}
	for _, folder := range configPath {
		if err := os.MkdirAll(folder, 0700); err != nil {
			return "", fmt.Errorf("create config folder %q: %w", folder, err)
		}
	}
	return configPath["ConfigFolder"], nil
}
