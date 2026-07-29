package agentui

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	ErrClosed                 = errors.New("agent ui is closed")
	ErrResultAlreadyDisplayed = errors.New("agent result was already displayed")
	ErrInvalidQuestion        = errors.New("question requires at least two non-empty options")
)

type Event interface {
	agentUIEvent()
}

type ActivityKind string

const (
	ActivityTool ActivityKind = "tool"
	ActivityFile ActivityKind = "file"
)

type TaskStatus string

const (
	TaskPending TaskStatus = "pending"
	TaskRunning TaskStatus = "running"
	TaskDone    TaskStatus = "done"
	TaskFailed  TaskStatus = "failed"
)

type TaskItem struct {
	Title  string
	Status TaskStatus
}

type UsageStats struct {
	TodayTokens    int64
	CacheHitRate   float64
	ContextUsed    float64
	ContextTotal   float64
}

type ResultEvent struct {
	Text string
}

func (ResultEvent) agentUIEvent() {}

type ThinkingEvent struct {
	Text string
}

func (ThinkingEvent) agentUIEvent() {}

type ActivityEvent struct {
	Kind   ActivityKind
	Target string
}

func (ActivityEvent) agentUIEvent() {}

type TaskPlanEvent struct {
	Tasks []TaskItem
}

func (TaskPlanEvent) agentUIEvent() {}

type UsageEvent struct {
	Stats UsageStats
}

func (UsageEvent) agentUIEvent() {}

type QuestionEvent struct {
	Question string
	Options  []string

	reply chan int
	once  sync.Once
}

func (*QuestionEvent) agentUIEvent() {}

func (event *QuestionEvent) Answer(index int) bool {
	if index < 0 || index >= len(event.Options) {
		return false
	}

	answered := false
	event.once.Do(func() {
		event.reply <- index
		answered = true
	})
	return answered
}

type AgentUI struct {
	events chan Event
	done   chan struct{}

	closeOnce sync.Once
	closed    atomic.Bool

	resultMu        sync.Mutex
	resultDisplayed bool
}

func New() *AgentUI {
	return &AgentUI{
		events: make(chan Event, 16),
		done:   make(chan struct{}),
	}
}

func (ui *AgentUI) Next() (Event, error) {
	if ui.closed.Load() {
		return nil, ErrClosed
	}
	select {
	case <-ui.done:
		return nil, ErrClosed
	default:
	}

	select {
	case <-ui.done:
		return nil, ErrClosed
	case event := <-ui.events:
		if ui.closed.Load() {
			return nil, ErrClosed
		}
		return event, nil
	}
}

func (ui *AgentUI) Close() {
	ui.closeOnce.Do(func() {
		ui.closed.Store(true)
		close(ui.done)
	})
}

// DisplayResult 将本轮 Agent 的最终回答发送给 TUI，每轮只允许调用一次。
func (ui *AgentUI) DisplayResult(text string) error {
	ui.resultMu.Lock()
	if ui.resultDisplayed {
		ui.resultMu.Unlock()
		return ErrResultAlreadyDisplayed
	}
	ui.resultDisplayed = true
	ui.resultMu.Unlock()

	return ui.send(ResultEvent{Text: text})
}

// Ask 在 TUI 中显示内联选择卡，并阻塞到用户确认选项或本轮被关闭。
func (ui *AgentUI) Ask(question string, options []string) (string, error) {
	if strings.TrimSpace(question) == "" || len(options) < 2 {
		return "", ErrInvalidQuestion
	}

	copiedOptions := append([]string(nil), options...)
	for _, option := range copiedOptions {
		if strings.TrimSpace(option) == "" {
			return "", ErrInvalidQuestion
		}
	}

	event := &QuestionEvent{
		Question: question,
		Options:  copiedOptions,
		reply:    make(chan int, 1),
	}
	if err := ui.send(event); err != nil {
		return "", err
	}

	select {
	case <-ui.done:
		return "", ErrClosed
	case index := <-event.reply:
		return event.Options[index], nil
	}
}

// DisplayThinking 将可展示的简短过程说明发送给 TUI；后续调用替换当前说明。
func (ui *AgentUI) DisplayThinking(text string) error {
	return ui.send(ThinkingEvent{Text: text})
}

// DisplayActivity 将工具调用或文件查看动作发送给 TUI 作为紧凑过程记录。
func (ui *AgentUI) DisplayActivity(kind ActivityKind, target string) error {
	return ui.send(ActivityEvent{
		Kind:   kind,
		Target: target,
	})
}

// DisplayTaskPlan 将最新任务编排快照发送给 TUI，用于刷新当前动态任务框。
func (ui *AgentUI) DisplayTaskPlan(tasks []TaskItem) error {
	return ui.send(TaskPlanEvent{
		Tasks: append([]TaskItem(nil), tasks...),
	})
}

// DisplayUsage 将最新的今日用量快照发送给 TUI。
func (ui *AgentUI) DisplayUsage(stats UsageStats) error {
	return ui.send(UsageEvent{Stats: stats})
}
func (ui *AgentUI) send(event Event) error {
	if ui.closed.Load() {
		return ErrClosed
	}
	select {
	case <-ui.done:
		return ErrClosed
	default:
	}

	select {
	case <-ui.done:
		return ErrClosed
	case ui.events <- event:
		if ui.closed.Load() {
			return ErrClosed
		}
		return nil
	}
}
