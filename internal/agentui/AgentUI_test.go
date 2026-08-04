package agentui

import (
	"errors"
	"testing"
)

func TestCloseCancelsAgentContext(t *testing.T) {
	ui := New()
	canceled := make(chan struct{})
	ui.SetCancel(func() { close(canceled) })
	ui.Close()
	select {
	case <-canceled:
	default:
		t.Fatal("Close did not cancel agent context")
	}
}

func TestDisplayResultAndFinalResultPublishDistinctEvents(t *testing.T) {
	ui := New()

	if err := ui.DisplayResult("first"); err != nil {
		t.Fatal(err)
	}
	if err := ui.DisplayResult("second"); err != nil {
		t.Fatal(err)
	}
	if err := ui.DisplayFinalResult("done"); err != nil {
		t.Fatal(err)
	}

	for index, want := range []struct {
		text  string
		final bool
	}{
		{text: "first"},
		{text: "second"},
		{text: "done", final: true},
	} {
		event, err := ui.Next()
		if err != nil {
			t.Fatal(err)
		}
		switch got := event.(type) {
		case ResultEvent:
			if want.final || got.Text != want.text {
				t.Fatalf("event %d = %#v, want result %q final=%v", index, got, want.text, want.final)
			}
		case FinalResultEvent:
			if !want.final || got.Text != want.text {
				t.Fatalf("event %d = %#v, want result %q final=%v", index, got, want.text, want.final)
			}
		default:
			t.Fatalf("event %d type = %T", index, event)
		}
	}
}

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
