package utils

import (
	"looporbit/internal/config"
	"os"
	"path/filepath"
	"runtime"
)

var (
	ProviderConfig    = "openai:completions" //ProviderCongig.go
	ConfigFolderPath  string
	ChatHistoryFolder string
	SessionId         string
	ApiKey            string
	BaseUrl           string
	Model             string
	Provider          string
	ThinkLevel        string
	Cwd               string
	MaxContextLength  float64 = 258000
	ProejctConfigPath string
	UserPath          string
	OS                string
)

func init() {
	CreateConfigPath, _ := config.GetConfigFolderPath()
	ConfigFolderPath = CreateConfigPath["ConfigFolder"]
	ChatHistoryFolder = CreateConfigPath["ChatHistoryFolder"]
	providerConfig, err := config.LoadDefaultProviderConfig()
	if err != nil {
		os.Remove(ConfigFolderPath)
	}
	ApiKey = providerConfig.ApiKey
	BaseUrl = providerConfig.BaseURL
	Model = providerConfig.Model
	Provider = providerConfig.Type
	mustReloadThinkLevelConfig()
	config.Cwd = Cwd
	ProejctConfigPath = filepath.Join(Cwd, ".looporbit")
	config.ProejctConfigPath = ProejctConfigPath
	UserPath, _ = config.GetUserHomePath()
	OS = runtime.GOOS
}
