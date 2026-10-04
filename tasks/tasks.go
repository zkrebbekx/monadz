// Package tasks runs many monadz.Task values at the same time.
//
// Every function here is structured: it returns only after every task that
// it started has stopped. When the outcome is known early (a failure in
// All, a success in Any, a first result in Race), the function cancels the
// context of the remaining tasks and waits for them.
//
// If a task panics, the function cancels the other tasks, waits for them,
// and then panics in the caller's goroutine with a *monadz.PanicError that
// holds the original stack trace.
package tasks

import (
	"context"
	"errors"
	"iter"

	"github.com/zkrebbekx/monadz"
)

// run starts the tasks with at most limit running at once (limit <= 0
// means no limit). After each task stops, stop reports whether the outcome
// is known. Then run cancels the rest, waits, and returns. Tasks that never
// started have no entry in started.
func run[T any](ctx context.Context, ts []monadz.Task[T], limit int, stop func(monadz.Result[T]) bool) (results []monadz.Result[T], started []bool, winner int) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	n := len(ts)
	results, started, winner = make([]monadz.Result[T], n), make([]bool, n), -1
	if limit <= 0 || limit > n {
		limit = n
	}
	futures := make([]*monadz.Future[T], n)
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

func await[T any](f *monadz.Future[T]) (r monadz.Result[T], p any) {
	defer func() { p = recover() }()
	return f.Await(), nil
}

func never[T any](monadz.Result[T]) bool { return false }

func failed[T any](r monadz.Result[T]) bool { return r.IsErr() }

// All runs the tasks at the same time and returns all values in input
// order. At the first failure, All cancels the other tasks, waits for them,
// and returns that failure. (Haskell: mapConcurrently; Rust: try_join_all.)
func All[T any](ts ...monadz.Task[T]) monadz.Task[[]T] {
	return Limit(0, ts...)
}

// Limit is All with at most n tasks running at once. n <= 0 means no limit.
func Limit[T any](n int, ts ...monadz.Task[T]) monadz.Task[[]T] {
	return monadz.TaskResult(func(ctx context.Context) monadz.Result[[]T] {
		results, _, winner := run(ctx, ts, n, failed[T])
		if winner >= 0 {
			return monadz.Err[[]T](results[winner].Err())
		}
		out := make([]T, len(results))
		for i, r := range results {
			out[i] = r.Must()
		}
		return monadz.Ok(out)
	})
}

// AllSettled runs the tasks at the same time, waits for every one, and
// returns every Result in input order. A failure does not cancel the others.
func AllSettled[T any](ts ...monadz.Task[T]) monadz.Task[[]monadz.Result[T]] {
	return monadz.TaskResult(func(ctx context.Context) monadz.Result[[]monadz.Result[T]] {
		results, _, _ := run(ctx, ts, 0, never[T])
		return monadz.Ok(results)
	})
}

// Any returns the first success and cancels the other tasks. If every task
// fails, Any returns all the errors joined, in input order.
// (Rust: select_ok; JavaScript: Promise.any.)
func Any[T any](ts ...monadz.Task[T]) monadz.Task[T] {
	return monadz.TaskResult(func(ctx context.Context) monadz.Result[T] {
		if len(ts) == 0 {
			return monadz.Err[T](errors.New("tasks: Any with no tasks"))
		}
		results, _, winner := run(ctx, ts, 0, monadz.Result[T].IsOk)
		if winner >= 0 {
			return results[winner]
		}
		errs := make([]error, len(results))
		for i, r := range results {
			errs[i] = r.Err()
		}
		return monadz.Err[T](errors.Join(errs...))
	})
}

// Race returns the first task to stop, success or failure, and cancels the
// others. (Haskell: race; Rust: select.)
func Race[T any](ts ...monadz.Task[T]) monadz.Task[T] {
	return monadz.TaskResult(func(ctx context.Context) monadz.Result[T] {
		if len(ts) == 0 {
			return monadz.Err[T](errors.New("tasks: Race with no tasks"))
		}
		results, _, winner := run(ctx, ts, 0, func(monadz.Result[T]) bool { return true })
		return results[winner]
	})
}

// Traverse applies f to each item with at most limit calls at once, and
// returns the results in input order. At the first failure, it cancels the
// calls in progress, starts no new calls, and returns that failure.
// limit <= 0 means no limit.
func Traverse[A, B any](items iter.Seq[A], limit int, f func(context.Context, A) (B, error)) monadz.Task[[]B] {
	return monadz.TaskResult(func(ctx context.Context) monadz.Result[[]B] {
		var ts []monadz.Task[B] // read items at Run, so each run reads the sequence again
		for a := range items {
			ts = append(ts, monadz.NewTask(func(ctx context.Context) (B, error) { return f(ctx, a) }))
		}
		return Limit(limit, ts...).Run(ctx)
	})
}

// Zip runs both tasks at the same time and returns both values. At the
// first failure, Zip cancels the other task and returns that failure.
func Zip[A, B any](a monadz.Task[A], b monadz.Task[B]) monadz.Task[monadz.Pair[A, B]] {
	return a.ZipWith(b, monadz.PairOf[A, B])
}

// Zip3 runs three tasks at the same time and returns all three values.
func Zip3[A, B, C any](a monadz.Task[A], b monadz.Task[B], c monadz.Task[C]) monadz.Task[monadz.Triple[A, B, C]] {
	ab := Zip(a, b)
	return ab.ZipWith(c, func(p monadz.Pair[A, B], v C) monadz.Triple[A, B, C] {
		return monadz.TripleOf(p.First, p.Second, v)
	})
}
