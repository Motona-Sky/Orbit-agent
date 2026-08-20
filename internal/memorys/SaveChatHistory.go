package memorys

import (
	"fmt"
	"os"
	"path/filepath"

	"orbit/internal/utils"
)

// SaveChatHistory 将序列化后的聊天历史以仅当前用户可读写的权限保存到指定会话文件。
func SaveChatHistory(memmessage []byte, sessionID string) error {
	path := filepath.Join(utils.ChatHistoryFolder, sessionID)
	if err := os.WriteFile(path, memmessage, 0600); err != nil {
		return fmt.Errorf("save session %q: %w", sessionID, err)
	}
	return nil
}

// CreateSessionFolder 确保会话目录存在，如果目录已存在则不做任何操作。
func CreateSessionFolder() {
	path := utils.ChatHistoryFolder
	os.MkdirAll(path, 0700)
}
