package agentui

import (
	"errors"
	"testing"
)

func TestDisplayUsagePublishesStatsEvent(t *testing.T) {
	ui := New()
	want := UsageStats{
		TodayTokens:  1234,
		CacheHitRate: 42.5,
	}

	if err := ui.DisplayUsage(want); err != nil {
		t.Fatal(err)
	}
	event, err := ui.Next()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := event.(UsageEvent)
	if !ok {
		t.Fatalf("event type = %T, want UsageEvent", event)
	}
	if got.Stats != want {
		t.Fatalf("event stats = %#v, want %#v", got.Stats, want)
	}
}

func TestDisplayUsageReturnsClosedError(t *testing.T) {
	ui := New()
	ui.Close()

	err := ui.DisplayUsage(UsageStats{})
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("DisplayUsage() error = %v, want ErrClosed", err)
	}
}
