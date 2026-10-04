package monadz

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrGoexit is the error of a Task whose goroutine called runtime.Goexit,
// for example through testing.T.FailNow.
var ErrGoexit = errors.New("monadz: task goroutine called runtime.Goexit")

// ErrZeroTask is the error from Run on a zero Task.
var ErrZeroTask = errors.New("monadz: Run on a zero Task; use NewTask")

// Task is a description of work that can fail and that takes a context.
// Nothing runs until you call Run or Start. You can run a Task many times;
// each run does the work again. That makes a Task the right input for
// retry, timeout, and concurrency combinators.
//
// Equivalents: Haskell ReaderT Context (ExceptT error IO) a, Rust async fn,
// Scala ZIO.
//
// Panics: Run does not recover panics, so a bug crashes in the caller's
// goroutine. Start, ZipWith, and the functions in package tasks run a Task
// in another goroutine. There, monadz captures a panic and raises it again
// in the goroutine that waits for the result. The message holds the original
// stack trace.
type Task[T any] struct {
	run func(context.Context) Result[T]
}

// NewTask returns a Task that calls f.
func NewTask[T any](f func(ctx context.Context) (T, error)) Task[T] {
	return Task[T]{run: func(ctx context.Context) Result[T] { return Of(f(ctx)) }}
}

// NewTaskResult returns a Task that calls f, which returns a Result.
func NewTaskResult[T any](f func(ctx context.Context) Result[T]) Task[T] {
	return Task[T]{run: f}
}

// TaskOk returns a Task that succeeds with v.
func TaskOk[T any](v T) Task[T] {
	return Task[T]{run: func(context.Context) Result[T] { return Ok(v) }}
}

// TaskErr returns a Task that fails with err.
func TaskErr[T any](err error) Task[T] {
	r := Err[T](err)
	return Task[T]{run: func(context.Context) Result[T] { return r }}
}

// Run runs the Task in the caller's goroutine. If ctx already ended, Run
// returns ctx.Err() and does not start the work.
func (t Task[T]) Run(ctx context.Context) Result[T] {
	if t.run == nil {
		return Err[T](ErrZeroTask)
	}
	if err := ctx.Err(); err != nil {
		return Err[T](err)
	}
	return t.run(ctx)
}

// Start runs the Task in a new goroutine and returns a Future for the
// result. Future.Cancel cancels the context that the Task receives.
func (t Task[T]) Start(ctx context.Context) *Future[T] {
	ctx, cancel := context.WithCancel(ctx)
	f := &Future[T]{done: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(f.done)
		defer cancel()
		finished := false
		defer func() {
			// runtime.Goexit (for example t.FailNow) runs the defers but skips
			// the assignment below. Report it as a failure, never as Ok(zero).
			if !finished {
				f.res = Err[T](ErrGoexit)
			}
		}()
		f.res, f.pe = runCapture(ctx, t)
		finished = true
	}()
	return f
}

func runCapture[T any](ctx context.Context, t Task[T]) (res Result[T], pe *PanicError) {
	defer func() {
		if p := recover(); p != nil {
			pe = asPanicError(p)
		}
	}()
	return t.Run(ctx), nil
}

// ---- Transformation ----

// Map applies f to the value.
func (t Task[T]) Map[U any](f func(T) U) Task[U] {
	return NewTaskResult(func(ctx context.Context) Result[U] { return t.Run(ctx).Map(f) })
}

// Then runs a Go-shaped step func(ctx, T) (U, error) after the Task.
func (t Task[T]) Then[U any](f func(context.Context, T) (U, error)) Task[U] {
	return NewTaskResult(func(ctx context.Context) Result[U] {
		return t.Run(ctx).Then(func(v T) (U, error) { return f(ctx, v) })
	})
}

// AndThen runs the Task that f returns. This is the monad bind.
func (t Task[T]) AndThen[U any](f func(T) Task[U]) Task[U] {
	return NewTaskResult(func(ctx context.Context) Result[U] {
		return t.Run(ctx).AndThen(func(v T) Result[U] { return f(v).Run(ctx) })
	})
}

// MapErr applies f to the error.
func (t Task[T]) MapErr(f func(error) error) Task[T] {
	return NewTaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).MapErr(f) })
}

// Wrap adds context to the error: "msg: <err>".
func (t Task[T]) Wrap(msg string) Task[T] {
	return NewTaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).Wrap(msg) })
}

// Wrapf is Wrap with a format string.
func (t Task[T]) Wrapf(format string, args ...any) Task[T] {
	return NewTaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).Wrapf(format, args...) })
}

// Ensure fails the Task with err if its value does not satisfy pred.
func (t Task[T]) Ensure(pred func(T) bool, err error) Task[T] {
	return NewTaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).Ensure(pred, err) })
}

// Inspect calls f with the value after a success.
func (t Task[T]) Inspect(f func(T)) Task[T] {
	return NewTaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).Inspect(f) })
}

// InspectErr calls f with the error after a failure.
func (t Task[T]) InspectErr(f func(error)) Task[T] {
	return NewTaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).InspectErr(f) })
}

// Recover handles a failure with a Go-shaped function.
func (t Task[T]) Recover(f func(error) (T, error)) Task[T] {
	return NewTaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).Recover(f) })
}

// OrElse runs the Task that f returns after a failure. Use it for a
// fallback source.
func (t Task[T]) OrElse(f func(error) Task[T]) Task[T] {
	return NewTaskResult(func(ctx context.Context) Result[T] {
		return t.Run(ctx).OrElse(func(err error) Result[T] { return f(err).Run(ctx) })
	})
}

// ZipWith runs both Tasks at the same time and combines the values with f.
// If one fails, ZipWith cancels the other, waits for it to stop, and returns
// the first failure. For a Pair, use tasks.Zip.
func (t Task[T]) ZipWith[U, V any](other Task[U], f func(T, U) V) Task[V] {
	return NewTaskResult(func(ctx context.Context) Result[V] {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		fa, fb := t.Start(ctx), other.Start(ctx)

		var (
			firstErr   error
			firstPanic func() // raises the first panic, in completion order
		)
		aDone, bDone := fa.done, fb.done
		for aDone != nil || bDone != nil {
			// Read a Future's fields only after its done channel is closed;
			// the other Future can still run and write its own fields.
			var panicked bool
			select {
			case <-aDone:
				aDone = nil
				panicked = fa.pe != nil
				if panicked && firstPanic == nil {
					firstPanic = func() { fa.Await() }
				}
				if !panicked && fa.res.err != nil && firstErr == nil {
					firstErr = fa.res.err
				}
			case <-bDone:
				bDone = nil
				panicked = fb.pe != nil
				if panicked && firstPanic == nil {
					firstPanic = func() { fb.Await() }
				}
				if !panicked && fb.res.err != nil && firstErr == nil {
					firstErr = fb.res.err
				}
			}
			if firstErr != nil || panicked {
				cancel()
			}
		}
		if firstPanic != nil {
			firstPanic() // the first panic is the bug; a later one is a consequence
		}
		a, b := fa.Await(), fb.Await()
		if firstErr != nil {
			return Err[V](firstErr)
		}
		return Ok(f(a.value, b.value))
	})
}

// ---- Resilience ----

// Timeout fails the Task with context.DeadlineExceeded if it does not finish
// within d. The Task's context is canceled at the deadline.
//
// Timeout returns at the deadline and does NOT wait for the work to stop,
// even if the work observes its context. Code in the work that runs after
// the deadline (cleanup, a late write) runs at the same time as the caller.
// This is the one exception to the structured-concurrency rule of package
// tasks. If the work panics after the deadline, the panic crashes the
// program, as an unrecovered goroutine panic does.
func (t Task[T]) Timeout(d time.Duration) Task[T] {
	return NewTaskResult(func(parent context.Context) Result[T] {
		ctx, cancel := context.WithTimeout(parent, d)
		defer cancel()
		f := t.Start(ctx)
		select {
		case <-f.done:
			return f.Await()
		case <-ctx.Done():
			// The work may still run. If it panics later, raise the panic in a
			// goroutine of its own, so that a bug still crashes the program.
			go f.Await()
			if err := parent.Err(); err != nil {
				return Err[T](err) // the caller's ctx ended; this is not a timeout
			}
			return Err[T](fmt.Errorf("monadz: task timed out after %v: %w", d, ctx.Err()))
		}
	})
}

// Delay waits for d, then runs the Task. The wait stops if ctx ends.
func (t Task[T]) Delay(d time.Duration) Task[T] {
	return NewTaskResult(func(ctx context.Context) Result[T] {
		if err := Sleep(ctx, d); err != nil {
			return Err[T](err)
		}
		return t.Run(ctx)
	})
}

// Once returns a Task that runs the work one time and then returns the
// cached result. Concurrent runs wait for the first one. A caller that waits
// stops waiting when its own context ends.
//
// A failure with a context error (context.Canceled or
// context.DeadlineExceeded, also from an inner Timeout) is not cached, so the
// next run tries again. A panic is cached: every later run panics with the
// same value, as sync.OnceValue does.
func (t Task[T]) Once() Task[T] {
	var (
		lock = make(chan struct{}, 1) // a mutex that a waiter can leave when its ctx ends
		done bool
		res  Result[T]
		pv   any // the cached panic value
	)
	return NewTaskResult(func(ctx context.Context) Result[T] {
		select {
		case lock <- struct{}{}:
		case <-ctx.Done():
			return Err[T](ctx.Err())
		}
		defer func() { <-lock }()
		if done {
			if pv != nil {
				panic(pv)
			}
			return res
		}
		defer func() {
			if p := recover(); p != nil {
				done, pv = true, p
				panic(p)
			}
		}()
		r := t.Run(ctx)
		if r.err != nil && (ctx.Err() != nil || isContextErr(r.err)) {
			return r
		}
		done, res = true, r
		return r
	})
}

// RetryPolicy says when and how often Retry runs a Task again.
type RetryPolicy struct {
	// MaxAttempts is the total number of runs, including the first.
	// A value below 1 means 1.
	MaxAttempts int
	// Backoff returns the wait before attempt n+1, after attempt n failed
	// (n starts at 1). Nil means no wait.
	Backoff func(attempt int) time.Duration
	// RetryIf reports whether an error is worth a retry. Nil means every
	// error. Retry always stops when its own ctx ends, so a nil RetryIf
	// still retries a timeout from an inner Timeout.
	RetryIf func(error) bool
}

// Retry runs the Task again after a failure, as the policy allows. The
// waits stop when ctx ends. The final error tells how many attempts ran.
func (t Task[T]) Retry(p RetryPolicy) Task[T] {
	return NewTaskResult(func(ctx context.Context) Result[T] {
		for attempt := 1; ; attempt++ {
			r := t.Run(ctx)
			if r.err == nil {
				return r
			}
			if attempt >= max(p.MaxAttempts, 1) || ctx.Err() != nil || !retryable(p, r.err) {
				return Err[T](fmt.Errorf("monadz: gave up after %d attempt(s): %w", attempt, r.err))
			}
			if p.Backoff != nil {
				if err := Sleep(ctx, p.Backoff(attempt)); err != nil {
					return Err[T](fmt.Errorf("monadz: retry stopped after %d attempt(s): %w", attempt, errors.Join(err, r.err)))
				}
			}
		}
	})
}

func retryable(p RetryPolicy, err error) bool {
	return p.RetryIf == nil || p.RetryIf(err)
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// ConstantBackoff waits d before every retry.
func ConstantBackoff(d time.Duration) func(int) time.Duration {
	return func(int) time.Duration { return d }
}

// ExponentialBackoff waits base, 2*base, 4*base, ..., up to limit.
func ExponentialBackoff(base, limit time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
		if base <= 0 || limit <= 0 {
			return max(0, min(base, limit))
		}
		d := base
		for i := 1; i < attempt && d < limit; i++ {
			if d > limit/2 {
				return limit // d*2 is above limit and can overflow
			}
			d *= 2
		}
		return min(d, limit)
	}
}

// Sleep waits for d, or until ctx ends. It returns ctx.Err() if ctx ends
// first.
func Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Future is a handle to a Task that runs in its own goroutine. Get one from
// Task.Start; the zero Future holds no Task.
type Future[T any] struct {
	done   chan struct{}
	cancel context.CancelFunc
	res    Result[T]
	pe     *PanicError
}

// Done returns a channel. Start closes it when the Task stops.
func (f *Future[T]) Done() <-chan struct{} { return f.done }

// Cancel cancels the context of the Task. The Task stops only if its work
// observes the context.
func (f *Future[T]) Cancel() {
	if f.cancel != nil {
		f.cancel()
	}
}

// Await waits for the Task and returns its Result. If the Task panicked,
// Await panics in the caller's goroutine with a *PanicError that holds the
// original stack trace.
func (f *Future[T]) Await() Result[T] {
	if f.done == nil {
		return Err[T](ErrZeroTask)
	}
	<-f.done
	if f.pe != nil {
		panic(&PanicError{Value: f.pe.Value, Stack: f.pe.Stack, rethrown: true})
	}
	return f.res
}

// AwaitContext is Await with a limit: it returns ctx.Err() if ctx ends
// first. It does not cancel the Task.
func (f *Future[T]) AwaitContext(ctx context.Context) Result[T] {
	select {
	case <-f.done:
		return f.Await()
	case <-ctx.Done():
		return Err[T](ctx.Err())
	}
}

// Poll returns the Result if the Task has stopped, else None. It does not
// wait. If the Task panicked, Poll panics as Await does. (Rust: Future::poll.)
func (f *Future[T]) Poll() Option[Result[T]] {
	select {
	case <-f.done:
		return Some(f.Await())
	default:
		return None[Result[T]]()
	}
}
