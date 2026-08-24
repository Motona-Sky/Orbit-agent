package hook

import (
	"fmt"
	"sync"
	"time"
)

// Event represents a hook event, used by both notification and filter hooks.
type Event struct {
	Name string         // Event name, e.g. "exec_command.before"
	Time time.Time      // Time when the event was triggered
	Data map[string]any // Payload data
}

// ───────── Notification hooks (Emit / On) ─────────

// Listener is a notification-only handler that receives events without returning a value.
type Listener func(event Event)

// ───────── Filter hooks (RunFilter / OnFilter) ─────────

// FilterResult is the return value of a filter handler.
type FilterResult struct {
	Allowed bool   // Whether the event is allowed to proceed
	Reason  string // Rejection reason (only meaningful when Allowed is false)
}

// Filter is a handler that can intercept an event and decide whether it should proceed.
type Filter func(event Event) (FilterResult, error)

// ───────── Global registry ─────────

var (
	mu        sync.RWMutex
	listeners = make(map[string][]Listener)
	filters   = make(map[string][]Filter)
)

// On registers a notification listener for the given event name.
func On(name string, fn Listener) {
	mu.Lock()
	defer mu.Unlock()
	listeners[name] = append(listeners[name], fn)
}

// OnFilter registers a filter for the given event name.
// Multiple filters are executed in registration order; if any filter denies, the whole chain is denied.
func OnFilter(name string, fn Filter) {
	mu.Lock()
	defer mu.Unlock()
	filters[name] = append(filters[name], fn)
}

// Emit fires a notification event. All registered listeners for the event name will be called.
func Emit(name string, data map[string]any) {
	mu.RLock()
	fns := listeners[name]
	mu.RUnlock()

	ev := Event{
		Name: name,
		Time: time.Now(),
		Data: data,
	}
	for _, fn := range fns {
		fn(ev)
	}
}

// RunFilter fires a filter event, executing all registered filters in order.
// Stops and returns immediately if any filter returns Allowed=false or an error.
// If no filters are registered, it defaults to allowed.
func RunFilter(name string, data map[string]any) (FilterResult, error) {
	mu.RLock()
	fns := filters[name]
	mu.RUnlock()

	if len(fns) == 0 {
		return FilterResult{Allowed: true}, nil
	}

	ev := Event{
		Name: name,
		Time: time.Now(),
		Data: data,
	}
	for _, fn := range fns {
		result, err := fn(ev)
		if err != nil {
			return FilterResult{}, err
		}
		if !result.Allowed {
			return result, nil
		}
	}
	return FilterResult{Allowed: true}, nil
}

// Reset clears all registered listeners and filters (intended for testing).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	listeners = make(map[string][]Listener)
	filters = make(map[string][]Filter)
}

// PrintListener is a sample notification listener that prints events to stdout.
func PrintListener(ev Event) {
	fmt.Printf("[HOOK] %s | %s | %v\n", ev.Time.Format("15:04:05"), ev.Name, ev.Data)
}
