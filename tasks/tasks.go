// Package tasks runs many lift.Task values at the same time.
//
// Every function here is structured: it returns only after every task that
// it started has stopped. When the outcome is known early (a failure in
// All, a success in Any, a first result in Race), the function cancels the
// context of the remaining tasks and waits for them.
//
// If a task panics, the function cancels the other tasks, waits for them,
// and then panics in the caller's goroutine with a *lift.PanicError that
// holds the original stack trace.
package tasks

import (
	"context"
	"errors"
	"iter"

	"github.com/zkrebbekx/lift"
)

// run starts the tasks with at most limit running at once (limit <= 0
// means no limit). After each task stops, stop reports whether the outcome
// is known. Then run cancels the rest, waits, and returns. Tasks that never
// started have no entry in started.
func run[T any](ctx context.Context, ts []lift.Task[T], limit int, stop func(lift.Result[T]) bool) (results []lift.Result[T], started []bool, winner int) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	n := len(ts)
	results, started, winner = make([]lift.Result[T], n), make([]bool, n), -1
	if limit <= 0 || limit > n {
		limit = n
	}
	futures := make([]*lift.Future[T], n)
	done := make(chan int, n)
	next, running := 0, 0
	launch := func() {
		i := next
		next++
		running++
		started[i] = true
		futures[i] = ts[i].Start(ctx)
		go func() { <-futures[i].Done(); done <- i }()
	}
	for next < limit {
		launch()
	}

	var firstPanic any
	for running > 0 {
		i := <-done
		running--
		r, p := await(futures[i])
		results[i] = r
		switch {
		case p != nil:
			if firstPanic == nil {
				firstPanic = p
				cancel()
			}
		case winner < 0 && firstPanic == nil && stop(r):
			winner = i
			cancel()
		}
		if winner < 0 && firstPanic == nil && next < n {
			launch()
		}
	}
	if firstPanic != nil {
		panic(firstPanic)
	}
	return results, started, winner
}

func await[T any](f *lift.Future[T]) (r lift.Result[T], p any) {
	defer func() { p = recover() }()
	return f.Await(), nil
}

func never[T any](lift.Result[T]) bool { return false }

func failed[T any](r lift.Result[T]) bool { return r.IsErr() }

// All runs the tasks at the same time and returns all values in input
// order. At the first failure, All cancels the other tasks, waits for them,
// and returns that failure. (Haskell: mapConcurrently; Rust: try_join_all.)
func All[T any](ts ...lift.Task[T]) lift.Task[[]T] {
	return Limit(0, ts...)
}

// Limit is All with at most n tasks running at once. n <= 0 means no limit.
func Limit[T any](n int, ts ...lift.Task[T]) lift.Task[[]T] {
	return lift.TaskResult(func(ctx context.Context) lift.Result[[]T] {
		results, _, winner := run(ctx, ts, n, failed[T])
		if winner >= 0 {
			return lift.Err[[]T](results[winner].Err())
		}
		out := make([]T, len(results))
		for i, r := range results {
			out[i] = r.Must()
		}
		return lift.Ok(out)
	})
}

// AllSettled runs the tasks at the same time, waits for every one, and
// returns every Result in input order. A failure does not cancel the others.
func AllSettled[T any](ts ...lift.Task[T]) lift.Task[[]lift.Result[T]] {
	return lift.TaskResult(func(ctx context.Context) lift.Result[[]lift.Result[T]] {
		results, _, _ := run(ctx, ts, 0, never[T])
		return lift.Ok(results)
	})
}

// Any returns the first success and cancels the other tasks. If every task
// fails, Any returns all the errors joined, in input order.
// (Rust: select_ok; JavaScript: Promise.any.)
func Any[T any](ts ...lift.Task[T]) lift.Task[T] {
	return lift.TaskResult(func(ctx context.Context) lift.Result[T] {
		if len(ts) == 0 {
			return lift.Err[T](errors.New("tasks: Any with no tasks"))
		}
		results, _, winner := run(ctx, ts, 0, lift.Result[T].IsOk)
		if winner >= 0 {
			return results[winner]
		}
		errs := make([]error, len(results))
		for i, r := range results {
			errs[i] = r.Err()
		}
		return lift.Err[T](errors.Join(errs...))
	})
}

// Race returns the first task to stop, success or failure, and cancels the
// others. (Haskell: race; Rust: select.)
func Race[T any](ts ...lift.Task[T]) lift.Task[T] {
	return lift.TaskResult(func(ctx context.Context) lift.Result[T] {
		if len(ts) == 0 {
			return lift.Err[T](errors.New("tasks: Race with no tasks"))
		}
		results, _, winner := run(ctx, ts, 0, func(lift.Result[T]) bool { return true })
		return results[winner]
	})
}

// Traverse applies f to each item with at most limit calls at once, and
// returns the results in input order. At the first failure, it cancels the
// calls in progress, starts no new calls, and returns that failure.
// limit <= 0 means no limit.
func Traverse[A, B any](items iter.Seq[A], limit int, f func(context.Context, A) (B, error)) lift.Task[[]B] {
	var ts []lift.Task[B]
	for a := range items {
		ts = append(ts, lift.NewTask(func(ctx context.Context) (B, error) { return f(ctx, a) }))
	}
	return Limit(limit, ts...)
}

// Zip runs both tasks at the same time and returns both values. At the
// first failure, Zip cancels the other task and returns that failure.
func Zip[A, B any](a lift.Task[A], b lift.Task[B]) lift.Task[lift.Pair[A, B]] {
	return a.ZipWith(b, lift.PairOf[A, B])
}

// Zip3 runs three tasks at the same time and returns all three values.
func Zip3[A, B, C any](a lift.Task[A], b lift.Task[B], c lift.Task[C]) lift.Task[lift.Triple[A, B, C]] {
	ab := Zip(a, b)
	return ab.ZipWith(c, func(p lift.Pair[A, B], v C) lift.Triple[A, B, C] {
		return lift.TripleOf(p.First, p.Second, v)
	})
}
