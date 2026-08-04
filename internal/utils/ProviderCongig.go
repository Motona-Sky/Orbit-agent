package utils

import "orbit/internal/config"

func UpdateProviderConfig(provider string) {
	ProviderConfig = provider
}

// ReloadProviderConfig 从当前默认 Provider 刷新下一轮请求使用的运行时配置。
func ReloadProviderConfig() error {
	providerConfig, err := config.LoadDefaultProviderConfig()
	if err != nil {
		return err
	}
	ApiKey = providerConfig.ApiKey
	BaseUrl = providerConfig.BaseURL
	Model = providerConfig.Model
	Provider = providerConfig.Type
	return nil
}
