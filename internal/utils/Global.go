package utils

import (
	"orbit/internal/config"
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
	Auth              string
	AccessToken       string
	AccountID         string
	RefreshToken      string
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

func GInit() {

	CreateConfigPath, _ := config.GetConfigFolderPath()
	// 检查配置目录是否已经初始化（此前错误地用未赋值的全局变量 ConfigFolderPath 即空串去 stat，
	// 导致永远进入 else 分支，ChatHistoryFolder 保持为空，会话被写入到当前工作目录）。
	_, err := os.Stat(CreateConfigPath["ConfigFolder"])
	if err == nil {

		ConfigFolderPath = CreateConfigPath["ConfigFolder"]
		ChatHistoryFolder = CreateConfigPath["ChatHistoryFolder"]
		providerConfig, err := config.LoadDefaultProviderConfig()
		if err != nil {
			os.Remove(ConfigFolderPath)
		}
		ApiKey = providerConfig.ApiKey
		Auth = providerConfig.Auth
		AccessToken = providerConfig.AccessToken
		AccountID = providerConfig.AccountID
		RefreshToken = providerConfig.RefreshToken
		BaseUrl = providerConfig.BaseURL
		Model = providerConfig.Model
		Provider = providerConfig.Type
		mustReloadThinkLevelConfig()
		config.Cwd = Cwd
		ProejctConfigPath = filepath.Join(Cwd, ".orbit")
		config.ProejctConfigPath = ProejctConfigPath
		UserPath, _ = config.GetUserHomePath()
		OS = runtime.GOOS
	} else {
	}
}
func init() {
	configpath, err := config.GetConfigFolderPath()
	Path := filepath.Join(configpath["ConfigFolder"], "Orbitconfig.yaml")
	if err != nil {
		panic(err)
	}
	_, err = os.Stat(Path)
	if err == nil {
		GInit()
	}

}
