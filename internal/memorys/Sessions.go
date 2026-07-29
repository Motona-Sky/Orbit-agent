package memorys

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"looporbit/internal/llm"
)

type SessionSummary struct {
	ID               string
	FirstUserMessage string
	ModifiedAt       time.Time
	Messages         []llm.MemoryMessage
}

// ListSessions 扫描会话目录，返回按最近修改时间倒序排列的有效会话及跳过的无效条目数量。
func ListSessions(dir string) ([]SessionSummary, int, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}

	sessions := make([]SessionSummary, 0, len(entries))
	skipped := 0
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			skipped++
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil || len(data) == 0 {
			skipped++
			continue
		}
		var messages []llm.MemoryMessage
		if err := json.Unmarshal(data, &messages); err != nil {
			skipped++
			continue
		}
		first := firstUserMessage(messages)
		if first == "" {
			skipped++
			continue
		}
		sessions = append(sessions, SessionSummary{
			ID:               entry.Name(),
			FirstUserMessage: first,
			ModifiedAt:       info.ModTime(),
			Messages:         messages,
		})
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].ModifiedAt.Equal(sessions[j].ModifiedAt) {
			return sessions[i].ID < sessions[j].ID
		}
		return sessions[i].ModifiedAt.After(sessions[j].ModifiedAt)
	})
	return sessions, skipped, nil
}

// firstUserMessage 返回首条非空用户消息，并将连续空白折叠为单个空格。
func firstUserMessage(messages []llm.MemoryMessage) string {
	for _, message := range messages {
		if message.Role == "user" {
			if content := strings.Join(strings.Fields(message.Content), " "); content != "" {
				return content
			}
		}
	}
	return ""
}
