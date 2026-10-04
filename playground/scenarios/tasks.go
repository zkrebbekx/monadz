package scenarios

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zkrebbekx/monadz"
	"github.com/zkrebbekx/monadz/tasks"
)

// ErrUpstream is the error of a task that the visitor set to fail.
var ErrUpstream = errors.New("upstream service failed")

// TaskSpec describes one task of the Tasks scenario.
type TaskSpec struct {
	Name       string `json:"name"`
	DurationMs int    `json:"durationMs"`
	Fails      bool   `json:"fails"`
}

// TasksInput is the input of the Tasks scenario.
type TasksInput struct {
	Mode  string     `json:"mode"` // all, any, race, allSettled
	Tasks []TaskSpec `json:"tasks"`
}

// Settled is one result of tasks.AllSettled.
type Settled struct {
	Task  string `json:"task"`
	Ok    bool   `json:"ok"`
	Value string `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
}

// TasksTrace is the result of the Tasks scenario.
type TasksTrace struct {
	Mode    string     `json:"mode"`
	Tasks   []TaskSpec `json:"tasks"`
	Events  []Event    `json:"events"`
	TotalMs int64      `json:"totalMs"`
	Ok      bool       `json:"ok"`
	Value   string     `json:"value,omitempty"`
	Error   string     `json:"error,omitempty"`
	Settled []Settled  `json:"settled,omitempty"`
}

// The scenario limits.
const (
	minTaskMs = 10
	maxTaskMs = 5000
)

// fetchTask is a Task that works for d, then succeeds or fails. It stops
// early, with a "canceled" event, if its context is canceled.
func fetchTask(spec TaskSpec, rec *recorder) monadz.Task[string] {
	return monadz.NewTask(func(ctx context.Context) (string, error) {
		rec.add(spec.Name, EventStart, "")
		if err := monadz.Sleep(ctx, time.Duration(spec.DurationMs)*time.Millisecond); err != nil {
			rec.add(spec.Name, EventCanceled, err.Error())
			return "", err
		}
		if spec.Fails {
			rec.add(spec.Name, EventFail, ErrUpstream.Error())
			return "", fmt.Errorf("%s: %w", spec.Name, ErrUpstream)
		}
		rec.add(spec.Name, EventOk, "")
		return spec.Name + " data", nil
	})
}

func normalizeSpecs(specs []TaskSpec) []TaskSpec {
	defaults := []string{"usage", "invoices", "tax rates"}
	if len(specs) > 3 {
		specs = specs[:3]
	}
	out := make([]TaskSpec, len(specs))
	for i, s := range specs {
		if s.Name == "" {
			s.Name = defaults[i]
		}
		s.DurationMs = clamp(s.DurationMs, minTaskMs, maxTaskMs)
		out[i] = s
	}
	return out
}

// RunTasks runs the tasks for real with the chosen combinator from package
// tasks, and records when each task starts and stops.
func RunTasks(ctx context.Context, in TasksInput) TasksTrace {
	specs := normalizeSpecs(in.Tasks)
	rec := newRecorder(ctx)
	ts := make([]monadz.Task[string], len(specs))
	for i, s := range specs {
		ts[i] = fetchTask(s, rec)
	}

	t := TasksTrace{Mode: in.Mode, Tasks: specs}
	switch in.Mode {
	case "any":
		setResult(&t, tasks.Any(ts...).Run(ctx), func(v string) string { return v })
	case "race":
		setResult(&t, tasks.Race(ts...).Run(ctx), func(v string) string { return v })
	case "allSettled":
		all := tasks.AllSettled(ts...).Run(ctx)
		t.Ok = true
		for i, r := range all.ValueOrZero() {
			s := Settled{Task: specs[i].Name, Ok: r.IsOk(), Value: r.ValueOrZero()}
			if err := r.Err(); err != nil {
				s.Error = err.Error()
			}
			t.Settled = append(t.Settled, s)
		}
		t.Value = fmt.Sprintf("%d results", len(t.Settled))
	default:
		t.Mode = "all"
		setResult(&t, tasks.All(ts...).Run(ctx), func(v []string) string { return fmt.Sprintf("%q", v) })
	}
	t.TotalMs = rec.elapsedMs()
	t.Events = rec.list()
	return t
}

func setResult[T any](t *TasksTrace, r monadz.Result[T], show func(T) string) {
	v, err := r.Get()
	if err != nil {
		t.Error = err.Error()
		return
	}
	t.Ok, t.Value = true, show(v)
}

// RetryInput is the input of the Retry scenario.
type RetryInput struct {
	FailuresBeforeSuccess int `json:"failuresBeforeSuccess"`
	MaxAttempts           int `json:"maxAttempts"`
	BaseMs                int `json:"baseMs"`
}

// Attempt is one run of the task inside Retry.
type Attempt struct {
	N       int    `json:"n"`
	StartMs int64  `json:"startMs"`
	EndMs   int64  `json:"endMs"`
	Ok      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

// RetryTrace is the result of the Retry scenario.
type RetryTrace struct {
	Attempts  []Attempt `json:"attempts"`
	Events    []Event   `json:"events"`
	PlanMs    []int64   `json:"planMs"` // the backoff before attempt n+1
	TotalMs   int64     `json:"totalMs"`
	Ok        bool      `json:"ok"`
	Value     string    `json:"value,omitempty"`
	Error     string    `json:"error,omitempty"`
	AttemptMs int       `json:"attemptMs"`
}

// ErrGatewayTimeout is the error of a failed charge attempt.
var ErrGatewayTimeout = errors.New("payment gateway timeout")

// retryAttemptMs is how long one charge attempt takes.
const retryAttemptMs = 150

// retryLimit is the longest backoff.
const retryLimit = 2 * time.Second

// RunRetry charges a card with Task.Retry and ExponentialBackoff. The charge
// fails the given number of times, then succeeds.
func RunRetry(ctx context.Context, in RetryInput) RetryTrace {
	failures := clamp(in.FailuresBeforeSuccess, 0, 8)
	maxAttempts := clamp(in.MaxAttempts, 1, 6)
	base := time.Duration(clamp(in.BaseMs, 10, 1000)) * time.Millisecond
	backoff := monadz.ExponentialBackoff(base, retryLimit)

	rec := newRecorder(ctx)
	var attempts []Attempt
	attempt := 0

	charge := monadz.NewTask(func(ctx context.Context) (string, error) {
		attempt++
		a := Attempt{N: attempt, StartMs: rec.elapsedMs()}
		name := fmt.Sprintf("attempt %d", attempt)
		rec.add(name, EventStart, "")
		defer func() { a.EndMs = rec.elapsedMs(); attempts = append(attempts, a) }()

		if err := monadz.Sleep(ctx, retryAttemptMs*time.Millisecond); err != nil {
			rec.add(name, EventCanceled, err.Error())
			a.Error = err.Error()
			return "", err
		}
		if attempt <= failures {
			rec.add(name, EventFail, ErrGatewayTimeout.Error())
			a.Error = ErrGatewayTimeout.Error()
			return "", ErrGatewayTimeout
		}
		rec.add(name, EventOk, "")
		a.Ok = true
		return "ch_3Pq9", nil
	}).Retry(monadz.RetryPolicy{
		MaxAttempts: maxAttempts,
		Backoff:     backoff,
	})

	res := charge.Run(ctx)

	t := RetryTrace{Attempts: attempts, Events: rec.list(), TotalMs: rec.elapsedMs(), AttemptMs: retryAttemptMs}
	for n := 1; n < maxAttempts; n++ {
		t.PlanMs = append(t.PlanMs, backoff(n).Milliseconds())
	}
	if v, err := res.Get(); err != nil {
		t.Error = err.Error()
	} else {
		t.Ok, t.Value = true, v
	}
	return t
}
