package memorys

import "looporbit/internal/llm"

// AppendMemoryMessages 按原始顺序将消息追加到记忆后面。
func AppendMemoryMessages(memory []llm.MemoryMessage, messages []llm.MemoryMessage) []llm.MemoryMessage {
	if len(memory) == 0 {
		return messages
	}
	if len(messages) == 0 {
		return memory
	}
	return append(memory, messages...)
}
