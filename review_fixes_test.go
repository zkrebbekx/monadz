package monadz_test

// Regression tests for the findings of the concurrency review.

import (
	"context"
	"errors"
	"math"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zkrebbekx/monadz"
	"github.com/zkrebbekx/monadz/tasks"
)

func fixRecover(f func()) (p any) {
	defer func() { p = recover() }()
	f()
	return nil
}

func TestGoexitIsNeverSuccess(t *testing.T) {
	t.Run("Given a Task whose goroutine calls runtime.Goexit", func(t *testing.T) {
		exiter := monadz.NewTask(func(context.Context) (int, error) { runtime.Goexit(); return 42, nil })

		t.Run("When it runs through Start and Await", func(t *testing.T) {
			r := exiter.Start(context.Background()).Await()
			t.Run("Then the Result is Err(ErrGoexit), not Ok(0)", func(t *testing.T) {
				if !errors.Is(r.Err(), monadz.ErrGoexit) {
					t.Fatalf("got %v, want ErrGoexit", r)
				}
			})
		})

		t.Run("When tasks.Any races it against a failure", func(t *testing.T) {
			r := tasks.Any(exiter, monadz.TaskErr[int](errors.New("real failure"))).Run(context.Background())
			t.Run("Then Any fails, and the Goexit is not a winner", func(t *testing.T) {
				if r.IsOk() {
					t.Fatalf("got %v, want a failure", r)
				}
				if !errors.Is(r.Err(), monadz.ErrGoexit) {
					t.Errorf("got %v, want the joined errors to hold ErrGoexit", r.Err())
				}
			})
		})
	})
}

func TestRetryRetriesAnInnerTimeout(t *testing.T) {
	t.Run("Given a Task whose first attempt is slower than its Timeout", func(t *testing.T) {
		var calls atomic.Int32
		task := monadz.NewTask(func(ctx context.Context) (int, error) {
			if calls.Add(1) == 1 {
				<-ctx.Done()
				return 0, ctx.Err()
			}
			return 7, nil
		}).Timeout(20 * time.Millisecond).Retry(monadz.RetryPolicy{MaxAttempts: 3})

		t.Run("When it runs with the default RetryIf", func(t *testing.T) {
			r := task.Run(context.Background())
			t.Run("Then the second attempt runs and succeeds", func(t *testing.T) {
				if v, err := r.Get(); err != nil || v != 7 || calls.Load() != 2 {
					t.Fatalf("got %v after %d calls, want Ok(7) after 2", r, calls.Load())
				}
			})
		})
	})

	t.Run("Given a Retry whose own ctx already ended", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var calls atomic.Int32
		r := monadz.NewTask(func(ctx context.Context) (int, error) {
			calls.Add(1)
			return 0, ctx.Err()
		}).Retry(monadz.RetryPolicy{MaxAttempts: 5}).Run(ctx)

		t.Run("Then Retry does not start the work", func(t *testing.T) {
			if calls.Load() != 0 || !errors.Is(r.Err(), context.Canceled) {
				t.Fatalf("got %v after %d calls, want context.Canceled after 0", r, calls.Load())
			}
		})
	})
}

func TestZipWithRaisesTheFirstPanic(t *testing.T) {
	t.Run("Given b panics first, and a panics later in its cleanup", func(t *testing.T) {
		a := monadz.NewTask(func(ctx context.Context) (int, error) { <-ctx.Done(); panic("second panic, in cleanup of a") })
		b := monadz.NewTask(func(context.Context) (int, error) { panic("first panic, in b") })

		t.Run("When tasks.Zip runs them", func(t *testing.T) {
			p := fixRecover(func() { tasks.Zip(a, b).Run(context.Background()) })
			t.Run("Then the caller gets the first panic", func(t *testing.T) {
				pe, ok := p.(*monadz.PanicError)
				if !ok || pe.Value != "first panic, in b" {
					t.Fatalf("got %#v, want *PanicError with the first panic", p)
				}
			})
		})
	})
}

func TestOnceCachesPanicsButNotContextErrors(t *testing.T) {
	t.Run("Given a Once task whose work panics", func(t *testing.T) {
		var runs atomic.Int32
		task := monadz.NewTask(func(context.Context) (int, error) { runs.Add(1); panic("boom") }).Once()

		t.Run("When three callers run it", func(t *testing.T) {
			var panics int
			for range 3 {
				if fixRecover(func() { task.Run(context.Background()) }) != nil {
					panics++
				}
			}
			t.Run("Then the work runs once and every caller panics", func(t *testing.T) {
				if runs.Load() != 1 || panics != 3 {
					t.Fatalf("got %d runs and %d panics, want 1 and 3", runs.Load(), panics)
				}
			})
		})
	})

	t.Run("Given a Once task whose first run hits an inner Timeout", func(t *testing.T) {
		var runs atomic.Int32
		task := monadz.NewTask(func(ctx context.Context) (int, error) {
			if runs.Add(1) == 1 {
				<-ctx.Done()
				return 0, ctx.Err()
			}
			return 9, nil
		}).Timeout(10 * time.Millisecond).Once()

		t.Run("When it runs two times", func(t *testing.T) {
			first := task.Run(context.Background())
			second := task.Run(context.Background())
			t.Run("Then the timeout is not cached, and the second run succeeds", func(t *testing.T) {
				if !errors.Is(first.Err(), context.DeadlineExceeded) {
					t.Errorf("first: got %v, want a deadline error", first)
				}
				if v, err := second.Get(); err != nil || v != 9 {
					t.Errorf("second: got %v, want Ok(9)", second)
				}
			})
		})
	})
}

func TestBackoffEdgeValues(t *testing.T) {
	cases := []struct {
		name        string
		base, limit time.Duration
		attempt     int
		want        time.Duration
	}{
		{"base 0", 0, time.Second, 1 << 30, 0},
		{"a negative base", -time.Millisecond, time.Second, 50, 0},
		{"a negative limit", time.Millisecond, -time.Second, 3, 0},
		{"a huge attempt", time.Nanosecond, math.MaxInt64, 200, math.MaxInt64},
		{"limit below base", time.Second, time.Millisecond, 1, time.Millisecond},
	}
	for _, c := range cases {
		t.Run("Given ExponentialBackoff with "+c.name, func(t *testing.T) {
			start := time.Now()
			got := monadz.ExponentialBackoff(c.base, c.limit)(c.attempt)
			t.Run("Then it returns a sane wait at once", func(t *testing.T) {
				if got != c.want {
					t.Errorf("got %v, want %v", got, c.want)
				}
				if el := time.Since(start); el > 50*time.Millisecond {
					t.Errorf("took %v, want O(log n)", el)
				}
			})
		})
	}
}

func TestFutureEdges(t *testing.T) {
	t.Run("Given a zero Future", func(t *testing.T) {
		var f monadz.Future[int]
		t.Run("When Await and Cancel run", func(t *testing.T) {
			f.Cancel()
			r := f.Await()
			t.Run("Then neither blocks nor panics, and Await returns ErrZeroTask", func(t *testing.T) {
				if !errors.Is(r.Err(), monadz.ErrZeroTask) {
					t.Fatalf("got %v, want ErrZeroTask", r)
				}
			})
		})
	})

	t.Run("Given a Future whose Task panicked", func(t *testing.T) {
		f := monadz.NewTask(func(context.Context) (int, error) { panic("x") }).Start(context.Background())
		<-f.Done()
		t.Run("When Poll runs", func(t *testing.T) {
			p := fixRecover(func() { f.Poll() })
			t.Run("Then Poll panics as Await does", func(t *testing.T) {
				pe, ok := p.(*monadz.PanicError)
				if !ok || !strings.Contains(pe.Error(), "original goroutine stack") {
					t.Fatalf("got %#v, want a rethrown *PanicError", p)
				}
			})
		})
	})
}

func TestTaskWrapfAndEnsure(t *testing.T) {
	ctx := context.Background()
	base := errors.New("down")

	t.Run("Given a failing Task", func(t *testing.T) {
		r := monadz.TaskErr[int](base).Wrapf("load tenant %s", "acme").Run(ctx)
		t.Run("Then Wrapf adds formatted context and keeps the cause", func(t *testing.T) {
			if r.Err().Error() != "load tenant acme: down" || !errors.Is(r.Err(), base) {
				t.Fatalf("got %v", r)
			}
		})
	})

	t.Run("Given a Task that succeeds with 3", func(t *testing.T) {
		errSmall := errors.New("too small")
		t.Run("When Ensure requires a value above 5", func(t *testing.T) {
			r := monadz.TaskOk(3).Ensure(func(n int) bool { return n > 5 }, errSmall).Run(ctx)
			t.Run("Then the Task fails with the given error", func(t *testing.T) {
				if !errors.Is(r.Err(), errSmall) {
					t.Fatalf("got %v, want %v", r, errSmall)
				}
			})
		})
		t.Run("When Ensure requires a value above 1", func(t *testing.T) {
			r := monadz.TaskOk(3).Ensure(func(n int) bool { return n > 1 }, errSmall).Run(ctx)
			t.Run("Then the value passes through", func(t *testing.T) {
				if v, err := r.Get(); err != nil || v != 3 {
					t.Fatalf("got %v, want Ok(3)", r)
				}
			})
		})
	})
}
