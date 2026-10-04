package monadz

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrZeroTask is the error from Run on a zero Task.
var ErrZeroTask = errors.New("monadz: Run on a zero Task; use NewTask")

// Task is a description of work that can fail and that takes a context.
// Nothing runs until you call Run or Start. You can run a Task many times;
// each run does the work again. That makes a Task the right input for
// retry, timeout, and concurrency combinators.
//
// Equivalents: Haskell IO (ExceptT e IO a), Rust async fn, Scala ZIO.
//
// Panics: Run does not recover panics, so a bug crashes in the caller's
// goroutine. When a Task runs in another goroutine (Start, ZipWith, and the
// functions in package tasks), a panic is captured and raised again in the
// goroutine that waits for the result, with the original stack trace in
// the message.
type Task[T any] struct {
	run func(context.Context) Result[T]
}

// NewTask returns a Task that calls f.
func NewTask[T any](f func(ctx context.Context) (T, error)) Task[T] {
	return Task[T]{run: func(ctx context.Context) Result[T] { return Of(f(ctx)) }}
}

// TaskResult returns a Task that calls f, which returns a Result.
func TaskResult[T any](f func(ctx context.Context) Result[T]) Task[T] {
	return Task[T]{run: f}
}

// TaskOf returns a Task that succeeds with v.
func TaskOf[T any](v T) Task[T] {
	return Task[T]{run: func(context.Context) Result[T] { return Ok(v) }}
}

// TaskErr returns a Task that fails with err.
func TaskErr[T any](err error) Task[T] {
	r := Err[T](err)
	return Task[T]{run: func(context.Context) Result[T] { return r }}
}

// Run runs the Task in the caller's goroutine. If ctx is already done, Run
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
		f.res, f.pe = runCapture(ctx, t)
	}()
	return f
}

func runCapture[T any](ctx context.Context, t Task[T]) (res Result[T], pe *PanicError) {
	defer func() {
		if p := recover(); p != nil {
			if existing, ok := p.(*PanicError); ok {
				pe = existing
			} else {
				pe = newPanicError(p)
			}
		}
	}()
	return t.Run(ctx), nil
}

// ---- Transformation ----

// Map applies f to the value.
func (t Task[T]) Map[U any](f func(T) U) Task[U] {
	return TaskResult(func(ctx context.Context) Result[U] { return t.Run(ctx).Map(f) })
}

// Then runs a Go-shaped step func(ctx, T) (U, error) after the Task.
func (t Task[T]) Then[U any](f func(context.Context, T) (U, error)) Task[U] {
	return TaskResult(func(ctx context.Context) Result[U] {
		v, err := t.Run(ctx).Get()
		if err != nil {
			return Err[U](err)
		}
		return Of(f(ctx, v))
	})
}

// AndThen runs the Task that f returns. This is the monad bind.
func (t Task[T]) AndThen[U any](f func(T) Task[U]) Task[U] {
	return TaskResult(func(ctx context.Context) Result[U] {
		v, err := t.Run(ctx).Get()
		if err != nil {
			return Err[U](err)
		}
		return f(v).Run(ctx)
	})
}

// MapErr applies f to the error.
func (t Task[T]) MapErr(f func(error) error) Task[T] {
	return TaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).MapErr(f) })
}

// Wrap adds context to the error: "msg: <err>".
func (t Task[T]) Wrap(msg string) Task[T] {
	return TaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).Wrap(msg) })
}

// Inspect calls f with the value after a success.
func (t Task[T]) Inspect(f func(T)) Task[T] {
	return TaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).Inspect(f) })
}

// InspectErr calls f with the error after a failure.
func (t Task[T]) InspectErr(f func(error)) Task[T] {
	return TaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).InspectErr(f) })
}

// Recover handles a failure with a Go-shaped function.
func (t Task[T]) Recover(f func(error) (T, error)) Task[T] {
	return TaskResult(func(ctx context.Context) Result[T] { return t.Run(ctx).Recover(f) })
}

// OrElse runs the Task that f returns after a failure. Use it for a
// fallback source.
func (t Task[T]) OrElse(f func(error) Task[T]) Task[T] {
	return TaskResult(func(ctx context.Context) Result[T] {
		r := t.Run(ctx)
		if r.err == nil {
			return r
		}
		return f(r.err).Run(ctx)
	})
}

// ZipWith runs both Tasks at the same time and combines the values with f.
// If one fails, ZipWith cancels the other, waits for it to stop, and returns
// the first failure. For a Pair, use tasks.Zip.
func (t Task[T]) ZipWith[U, V any](other Task[U], f func(T, U) V) Task[V] {
	return TaskResult(func(ctx context.Context) Result[V] {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		fa, fb := t.Start(ctx), other.Start(ctx)

		var firstErr error
		aDone, bDone := fa.done, fb.done
		for aDone != nil || bDone != nil {
			// Read a Future's fields only after its done channel is closed;
			// the other Future can still run and write its own fields.
			var panicked bool
			select {
			case <-aDone:
				aDone = nil
				panicked = fa.pe != nil
				if !panicked && fa.res.err != nil && firstErr == nil {
					firstErr = fa.res.err
				}
			case <-bDone:
				bDone = nil
				panicked = fb.pe != nil
				if !panicked && fb.res.err != nil && firstErr == nil {
					firstErr = fb.res.err
				}
			}
			if firstErr != nil || panicked {
				cancel()
			}
		}
		a, b := fa.Await(), fb.Await() // raises a captured panic
		if firstErr != nil {
			return Err[V](firstErr)
		}
		return Ok(f(a.value, b.value))
	})
}

// ---- Resilience ----

// Timeout fails the Task with context.DeadlineExceeded if it does not finish
// within d. The Task's context is canceled at the deadline. If the work
// ignores its context, Timeout still returns at the deadline, and the work
// finishes in the background. If that background work panics, the panic
// crashes the program, as an unrecovered goroutine panic does.
func (t Task[T]) Timeout(d time.Duration) Task[T] {
	return TaskResult(func(parent context.Context) Result[T] {
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

// Delay waits for d, then runs the Task. The wait stops if ctx is done.
func (t Task[T]) Delay(d time.Duration) Task[T] {
	return TaskResult(func(ctx context.Context) Result[T] {
		if err := Sleep(ctx, d); err != nil {
			return Err[T](err)
		}
		return t.Run(ctx)
	})
}

// Once returns a Task that runs the work one time and then returns the
// cached result. Concurrent runs wait for the first one. A run that fails
// because its own context ended is not cached, so the next run tries again.
// A caller that waits for another run stops waiting when its own context ends.
func (t Task[T]) Once() Task[T] {
	var (
		lock = make(chan struct{}, 1) // a mutex that a waiter can leave when its ctx ends
		done bool
		res  Result[T]
	)
	return TaskResult(func(ctx context.Context) Result[T] {
		select {
		case lock <- struct{}{}:
		case <-ctx.Done():
			return Err[T](ctx.Err())
		}
		defer func() { <-lock }()
		if done {
			return res
		}
		r := t.Run(ctx)
		if r.err != nil && ctx.Err() != nil {
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
	// error except a context error.
	RetryIf func(error) bool
}

// Retry runs the Task again after a failure, as the policy allows. The
// waits stop when ctx is done. The final error tells how many attempts ran.
func (t Task[T]) Retry(p RetryPolicy) Task[T] {
	return TaskResult(func(ctx context.Context) Result[T] {
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
	if p.RetryIf != nil {
		return p.RetryIf(err)
	}
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// ConstantBackoff waits d before every retry.
func ConstantBackoff(d time.Duration) func(int) time.Duration {
	return func(int) time.Duration { return d }
}

// ExponentialBackoff waits base, 2*base, 4*base, ..., up to limit.
func ExponentialBackoff(base, limit time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
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

// Sleep waits for d, or until ctx is done. It returns ctx.Err() if ctx ends
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

// Future is a handle to a Task that runs in its own goroutine.
type Future[T any] struct {
	done   chan struct{}
	cancel context.CancelFunc
	res    Result[T]
	pe     *PanicError
}

// Done returns a channel that is closed when the Task stops.
func (f *Future[T]) Done() <-chan struct{} { return f.done }

// Cancel cancels the context of the Task. The Task stops only if its work
// observes the context.
func (f *Future[T]) Cancel() { f.cancel() }

// Await waits for the Task and returns its Result. If the Task panicked,
// Await panics in the caller's goroutine with a *PanicError that holds the
// original stack trace.
func (f *Future[T]) Await() Result[T] {
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
// wait. (Rust: Future::poll.)
func (f *Future[T]) Poll() Option[Result[T]] {
	select {
	case <-f.done:
		return Some(f.Await())
	default:
		return None[Result[T]]()
	}
}
