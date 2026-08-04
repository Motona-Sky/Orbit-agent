package memorys

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"orbit/internal/llm"
	"orbit/internal/utils"
)

// LoadMemory 从历史目录读取并解析指定会话，读取或 JSON 解析失败时返回带会话 ID 的错误。
func LoadMemory(sessionID string) ([]llm.MemoryMessage, error) {
	path := filepath.Join(utils.ChatHistoryFolder, sessionID)
	rawMemory, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session %q: %w", sessionID, err)
	}
	var messages []llm.MemoryMessage
	if err := json.Unmarshal(rawMemory, &messages); err != nil {
		return nil, fmt.Errorf("parse session %q: %w", sessionID, err)
	}
	return messages, nil
}
