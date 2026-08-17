package event

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"orbit/internal/debug"
	"orbit/internal/utils"
)

type SessionId struct {
	Pwd       string
	SessionId string
}

// NewSessionId 根据工作目录摘要和安全随机数生成新的会话 ID。
func NewSessionId(pwd string) string {
	pwdHash := sha256.Sum256([]byte(pwd))

	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		panic("generate session id: " + err.Error())
	}
	return hex.EncodeToString(pwdHash[:2]) + "-" +
		hex.EncodeToString(randomBytes[:2]) + "-" +
		hex.EncodeToString(randomBytes[2:4]) + "-" +
		hex.EncodeToString(randomBytes[4:6])
}

var chatTuiInit SessionId

// 直接从TuiChat Init中运行，获取sessionid
func InitChatTuiEvent(pwd string) SessionId {
	sessionID := NewSessionId(pwd)
	if sessionID == "" {
		panic("generate session id failed")
	}
	return setChatTuiEvent(pwd, sessionID)
}

func ResumeChatTuiEvent(pwd, sessionID string) SessionId {
	if sessionID == "" {
		panic("resume session id is empty")
	}
	return setChatTuiEvent(pwd, sessionID)
}

// setChatTuiEvent 统一更新事件包和全局工具包中的当前会话状态。
func setChatTuiEvent(pwd, sessionID string) SessionId {
	chatTuiInit = SessionId{Pwd: pwd, SessionId: sessionID}
	utils.SessionId = sessionID
	utils.Cwd = pwd
	if err := debug.StartSession(pwd, sessionID); err != nil {
		panic("start debug session: " + err.Error())
	}
	return chatTuiInit
}
