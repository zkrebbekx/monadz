// Package scenarios holds the stories that the monadz playground shows.
//
// Each scenario is a pure function. It takes a small input struct, calls
// the real monadz API, and returns a trace that the browser can draw. The
// package has no build tags, so the normal tests cover it. The WASM bridge
// in ../wasm only converts JSON.
package scenarios

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Track values for a Step.
const (
	TrackOk      = "ok"
	TrackErr     = "err"
	TrackSkipped = "skipped"
	TrackNone    = "none"
	TrackSome    = "some"
	TrackLeft    = "left"
	TrackRight   = "right"
	TrackPanic   = "panic"
)

// Step is one stage of a pipeline.
type Step struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Output string `json:"output"`
	Track  string `json:"track"`
	Note   string `json:"note,omitempty"`
}

// Event kinds for a timeline.
const (
	EventStart    = "start"
	EventOk       = "ok"
	EventFail     = "fail"
	EventCanceled = "canceled"
	EventPanic    = "panic"
)

// Event is one point on a timeline. AtMs counts from the start of the run.
type Event struct {
	Task string `json:"task"`
	Kind string `json:"kind"`
	AtMs int64  `json:"atMs"`
	Note string `json:"note,omitempty"`
}

type hookKey struct{}

// WithEventHook returns a context that sends each timeline event to hook
// when it happens. The playground uses it to draw a run live. The hook can
// be called from many goroutines, one call at a time.
func WithEventHook(ctx context.Context, hook func(Event)) context.Context {
	return context.WithValue(ctx, hookKey{}, hook)
}

// recorder collects timeline events from many goroutines.
type recorder struct {
	mu     sync.Mutex
	start  time.Time
	events []Event
	hook   func(Event)
}

func newRecorder(ctx context.Context) *recorder {
	hook, _ := ctx.Value(hookKey{}).(func(Event))
	return &recorder{start: time.Now(), hook: hook}
}

func (r *recorder) add(task, kind, note string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := Event{Task: task, Kind: kind, AtMs: time.Since(r.start).Milliseconds(), Note: note}
	r.events = append(r.events, e)
	if r.hook != nil {
		r.hook(e)
	}
}

func (r *recorder) elapsedMs() int64 { return time.Since(r.start).Milliseconds() }

func (r *recorder) list() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}

// lookupIn turns a map into the func(K) (V, bool) shape that Option.Then
// accepts.
func lookupIn[K comparable, V any](m map[K]V) func(K) (V, bool) {
	return func(k K) (V, bool) {
		v, ok := m[k]
		return v, ok
	}
}

// quote formats a string for display in a step box.
func quote(s string) string { return fmt.Sprintf("%q", s) }

// clamp limits n to the range [lo, hi].
func clamp(n, lo, hi int) int { return max(lo, min(n, hi)) }
