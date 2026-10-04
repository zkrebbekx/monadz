package monadz_test

import (
	"context"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zkrebbekx/monadz"
)

// tskCounter returns a Task that fails with errs[n-1] on run n (from 1) and
// succeeds with the run number when errs has no more entries.
func tskCounter(calls *atomic.Int32, errs ...error) monadz.Task[int] {
	return monadz.NewTask(func(context.Context) (int, error) {
		n := int(calls.Add(1))
		if n <= len(errs) {
			return 0, errs[n-1]
		}
		return n, nil
	})
}

// tskContains fails the test if s does not contain sub.
func tskContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("got %q, want it to contain %q", s, sub)
	}
}

func TestTaskTimeout(t *testing.T) {
	ctx := context.Background()

	t.Run("Given a Task that waits for its ctx", func(t *testing.T) {
		t.Run("When Timeout is 20ms", func(t *testing.T) {
			started, stopped := make(chan struct{}), make(chan struct{})
			start := time.Now()
			r := tskWaitCtx(started, stopped).Timeout(20 * time.Millisecond).Run(ctx)
			elapsed := time.Since(start)
			t.Run("Then the error matches DeadlineExceeded and the work's ctx is canceled", func(t *testing.T) {
				tskIs(t, r.Err(), context.DeadlineExceeded)
				if elapsed >= time.Second {
					t.Errorf("got %v, want < 1s", elapsed)
				}
				if !tskClosed(started) {
					return // the work never started, so nothing can still run
				}
				select {
				case <-stopped:
				case <-time.After(5 * time.Second):
					t.Fatal("got a running Task, want its ctx canceled")
				}
			})
		})
	})

	t.Run("Given a Task that ignores its ctx", func(t *testing.T) {
		release := make(chan struct{})
		finished := make(chan struct{})
		tk := monadz.NewTask(func(context.Context) (int, error) {
			defer close(finished)
			<-release
			return 1, nil
		})
		t.Run("When Timeout is 20ms", func(t *testing.T) {
			start := time.Now()
			r := tk.Timeout(20 * time.Millisecond).Run(ctx)
			elapsed := time.Since(start)
			close(release)
			<-finished
			t.Run("Then Timeout still returns at the deadline", func(t *testing.T) {
				tskIs(t, r.Err(), context.DeadlineExceeded)
				tskContains(t, r.Err().Error(), "timed out after 20ms")
				if elapsed >= time.Second {
					t.Errorf("got %v, want < 1s", elapsed)
				}
			})
		})
	})

	t.Run("Given a Task that ignores its ctx and a 5s Timeout", func(t *testing.T) {
		release := make(chan struct{})
		finished := make(chan struct{})
		tk := monadz.NewTask(func(context.Context) (int, error) {
			defer close(finished)
			<-release
			return 1, nil
		}).Timeout(5 * time.Second)
		t.Run("When the caller cancels its ctx after 10ms", func(t *testing.T) {
			c, cancel := context.WithCancel(ctx)
			time.AfterFunc(10*time.Millisecond, cancel)
			start := time.Now()
			r := tk.Run(c)
			elapsed := time.Since(start)
			close(release)
			<-finished
			t.Run("Then it returns context.Canceled, not a timeout", func(t *testing.T) {
				tskIs(t, r.Err(), context.Canceled)
				if strings.Contains(r.Err().Error(), "timed out") {
					t.Errorf("got %q, want no timeout message", r.Err())
				}
				if elapsed >= time.Second {
					t.Errorf("got %v, want < 1s", elapsed)
				}
			})
		})
	})

	t.Run("Given a Task that finishes in time", func(t *testing.T) {
		t.Run("When Timeout is 1 minute", func(t *testing.T) {
			r := monadz.TaskOk(5).Timeout(time.Minute).Run(ctx)
			t.Run("Then the value returns", func(t *testing.T) { tskEq(t, "value", r.Must(), 5) })
		})
	})

	t.Run("Given a Task that panics in time", func(t *testing.T) {
		t.Run("When Timeout is 1 minute", func(t *testing.T) {
			p := tskMustPanic(t, func() {
				monadz.NewTask(func(context.Context) (int, error) { panic("t") }).Timeout(time.Minute).Run(ctx)
			})
			t.Run("Then the caller gets a *PanicError", func(t *testing.T) { tskPanicError(t, p, "t") })
		})
	})
}

func TestTaskDelay(t *testing.T) {
	t.Run("Given a Task with a 10ms Delay", func(t *testing.T) {
		t.Run("When the delay ends", func(t *testing.T) {
			start := time.Now()
			r := monadz.TaskOk(1).Delay(10 * time.Millisecond).Run(context.Background())
			elapsed := time.Since(start)
			t.Run("Then the work runs after the wait", func(t *testing.T) {
				tskEq(t, "value", r.Must(), 1)
				if elapsed < 10*time.Millisecond {
					t.Errorf("got %v, want >= 10ms", elapsed)
				}
			})
		})
	})

	t.Run("Given a Task with a 5s Delay", func(t *testing.T) {
		t.Run("When ctx is canceled during the wait", func(t *testing.T) {
			var calls atomic.Int32
			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(10*time.Millisecond, cancel)
			start := time.Now()
			r := tskCounter(&calls).Delay(5 * time.Second).Run(ctx)
			elapsed := time.Since(start)
			t.Run("Then the wait stops early and the work does not run", func(t *testing.T) {
				tskIs(t, r.Err(), context.Canceled)
				if elapsed >= time.Second {
					t.Errorf("got %v, want < 1s", elapsed)
				}
				tskEq(t, "calls", calls.Load(), int32(0))
			})
		})
	})
}

func TestTaskOnce(t *testing.T) {
	ctx := context.Background()

	t.Run("Given a counting Task wrapped with Once", func(t *testing.T) {
		t.Run("When it runs three times", func(t *testing.T) {
			var calls atomic.Int32
			tk := tskCounter(&calls).Once()
			got := []int{tk.Run(ctx).Must(), tk.Run(ctx).Must(), tk.Run(ctx).Must()}
			t.Run("Then the work runs once and the result is cached", func(t *testing.T) {
				tskEq(t, "calls", calls.Load(), int32(1))
				tskEq(t, "values", got, []int{1, 1, 1})
			})
		})

		t.Run("When it runs from 32 goroutines at the same time", func(t *testing.T) {
			var calls atomic.Int32
			tk := tskCounter(&calls).Once()
			var wg sync.WaitGroup
			got := make([]int, 32)
			for i := range got {
				wg.Go(func() { got[i] = tk.Run(ctx).Must() })
			}
			wg.Wait()
			t.Run("Then the work runs once and every run gets the same value", func(t *testing.T) {
				tskEq(t, "calls", calls.Load(), int32(1))
				for i, v := range got {
					if v != 1 {
						t.Errorf("run %d: got %d, want 1", i, v)
					}
				}
			})
		})
	})

	t.Run("Given a Task that fails with its own error, wrapped with Once", func(t *testing.T) {
		t.Run("When it runs twice", func(t *testing.T) {
			var calls atomic.Int32
			tk := tskCounter(&calls, tskErrBoom).Once()
			a, b := tk.Run(ctx), tk.Run(ctx)
			t.Run("Then the failure is cached", func(t *testing.T) {
				tskEq(t, "calls", calls.Load(), int32(1))
				if a.Err() != tskErrBoom || b.Err() != tskErrBoom {
					t.Errorf("got %v and %v, want %v", a.Err(), b.Err(), tskErrBoom)
				}
			})
		})
	})

	t.Run("Given a Once Task whose first run waits for its ctx", func(t *testing.T) {
		var calls atomic.Int32
		tk := monadz.NewTask(func(ctx context.Context) (int, error) {
			n := int(calls.Add(1))
			if n == 1 {
				<-ctx.Done()
				return 0, ctx.Err()
			}
			return n, nil
		}).Once()
		t.Run("When the caller cancels the first run, then runs twice more", func(t *testing.T) {
			c, cancel := context.WithCancel(ctx)
			time.AfterFunc(5*time.Millisecond, cancel)
			first := tk.Run(c)
			second, third := tk.Run(ctx), tk.Run(ctx)
			t.Run("Then the canceled failure is not cached", func(t *testing.T) {
				tskIs(t, first.Err(), context.Canceled)
				tskEq(t, "second", second.Must(), 2)
				tskEq(t, "third", third.Must(), 2)
				tskEq(t, "calls", calls.Load(), int32(2))
			})
		})
	})

	t.Run("Given a Once Task", func(t *testing.T) {
		var calls atomic.Int32
		tk := tskCounter(&calls).Once()
		t.Run("When the first run gets a ctx that is already canceled", func(t *testing.T) {
			c, cancel := context.WithCancel(ctx)
			cancel()
			first := tk.Run(c)
			second := tk.Run(ctx)
			t.Run("Then the work does not run and the next run tries", func(t *testing.T) {
				tskIs(t, first.Err(), context.Canceled)
				tskEq(t, "second", second.Must(), 1)
			})
		})
	})
}

func TestTaskRetry(t *testing.T) {
	ctx := context.Background()

	t.Run("Given a Task that fails twice, then succeeds", func(t *testing.T) {
		t.Run("When Retry allows 5 attempts", func(t *testing.T) {
			var calls atomic.Int32
			var attempts []int
			r := tskCounter(&calls, tskErrBoom, tskErrBoom).Retry(monadz.RetryPolicy{
				MaxAttempts: 5,
				Backoff:     func(n int) time.Duration { attempts = append(attempts, n); return 0 },
			}).Run(ctx)
			t.Run("Then it succeeds on run 3 and Backoff gets 1 and 2", func(t *testing.T) {
				tskEq(t, "value", r.Must(), 3)
				tskEq(t, "calls", calls.Load(), int32(3))
				tskEq(t, "backoff attempts", attempts, []int{1, 2})
			})
		})
	})

	t.Run("Given a Task that always fails", func(t *testing.T) {
		t.Run("When Retry allows 3 attempts", func(t *testing.T) {
			var calls atomic.Int32
			r := tskCounter(&calls, tskErrBoom, tskErrBoom, tskErrBoom, tskErrBoom).
				Retry(monadz.RetryPolicy{MaxAttempts: 3}).Run(ctx)
			t.Run("Then the error tells 3 attempts and reaches the cause", func(t *testing.T) {
				tskEq(t, "calls", calls.Load(), int32(3))
				tskContains(t, r.Err().Error(), "after 3 attempt(s)")
				tskIs(t, r.Err(), tskErrBoom)
			})
		})

		for _, n := range []int{1, 0, -3} {
			t.Run("When MaxAttempts is "+map[int]string{1: "1", 0: "0", -3: "-3"}[n], func(t *testing.T) {
				var calls atomic.Int32
				r := tskCounter(&calls, tskErrBoom, tskErrBoom).Retry(monadz.RetryPolicy{MaxAttempts: n}).Run(ctx)
				t.Run("Then the work runs exactly once", func(t *testing.T) {
					tskEq(t, "calls", calls.Load(), int32(1))
					tskContains(t, r.Err().Error(), "after 1 attempt(s)")
				})
			})
		}

		t.Run("When RetryIf returns false", func(t *testing.T) {
			var calls atomic.Int32
			var seen error
			r := tskCounter(&calls, tskErrBoom, tskErrBoom).Retry(monadz.RetryPolicy{
				MaxAttempts: 5,
				RetryIf:     func(err error) bool { seen = err; return false },
			}).Run(ctx)
			t.Run("Then it stops at once", func(t *testing.T) {
				tskEq(t, "calls", calls.Load(), int32(1))
				if seen != tskErrBoom {
					t.Errorf("RetryIf got %v, want %v", seen, tskErrBoom)
				}
				tskIs(t, r.Err(), tskErrBoom)
			})
		})
	})

	t.Run("Given a Task that returns a context error once", func(t *testing.T) {
		for _, ce := range []error{context.Canceled, context.DeadlineExceeded} {
			t.Run("When the error is "+ce.Error()+" and RetryIf is the default", func(t *testing.T) {
				var calls atomic.Int32
				r := tskCounter(&calls, ce).Retry(monadz.RetryPolicy{MaxAttempts: 5}).Run(ctx)
				t.Run("Then it retries, because the caller's ctx did not end", func(t *testing.T) {
					tskEq(t, "calls", calls.Load(), int32(2))
					if r.IsErr() {
						t.Errorf("got %v, want Ok after the retry", r)
					}
				})
			})
		}
		t.Run("When RetryIf always returns true", func(t *testing.T) {
			var calls atomic.Int32
			r := tskCounter(&calls, context.DeadlineExceeded).Retry(monadz.RetryPolicy{
				MaxAttempts: 3,
				RetryIf:     func(error) bool { return true },
			}).Run(ctx)
			t.Run("Then the custom rule is used and it retries", func(t *testing.T) {
				tskEq(t, "value", r.Must(), 2)
			})
		})
	})

	t.Run("Given a failing Task with a 5s backoff", func(t *testing.T) {
		t.Run("When ctx is canceled during the backoff", func(t *testing.T) {
			var calls atomic.Int32
			c, cancel := context.WithCancel(ctx)
			time.AfterFunc(10*time.Millisecond, cancel)
			start := time.Now()
			r := tskCounter(&calls, tskErrBoom, tskErrBoom).Retry(monadz.RetryPolicy{
				MaxAttempts: 5,
				Backoff:     monadz.ConstantBackoff(5 * time.Second),
			}).Run(c)
			elapsed := time.Since(start)
			t.Run("Then the retry stops early and reaches both errors", func(t *testing.T) {
				if elapsed >= time.Second {
					t.Errorf("got %v, want < 1s", elapsed)
				}
				tskEq(t, "calls", calls.Load(), int32(1))
				tskContains(t, r.Err().Error(), "after 1 attempt(s)")
				tskIs(t, r.Err(), context.Canceled)
				tskIs(t, r.Err(), tskErrBoom)
			})
		})
	})

	t.Run("Given a failing Task that cancels the caller's ctx", func(t *testing.T) {
		t.Run("When Retry allows 5 attempts", func(t *testing.T) {
			var calls atomic.Int32
			c, cancel := context.WithCancel(ctx)
			r := monadz.NewTask(func(context.Context) (int, error) {
				calls.Add(1)
				cancel()
				return 0, tskErrBoom
			}).Retry(monadz.RetryPolicy{MaxAttempts: 5, RetryIf: func(error) bool { return true }}).Run(c)
			t.Run("Then no further attempt runs", func(t *testing.T) {
				tskEq(t, "calls", calls.Load(), int32(1))
				tskIs(t, r.Err(), tskErrBoom)
			})
		})
	})
}

func TestBackoff(t *testing.T) {
	t.Run("Given ConstantBackoff(3ms)", func(t *testing.T) {
		b := monadz.ConstantBackoff(3 * time.Millisecond)
		t.Run("When it is called for attempts 1, 2, and 10", func(t *testing.T) {
			got := []time.Duration{b(1), b(2), b(10)}
			t.Run("Then every wait is 3ms", func(t *testing.T) {
				want := 3 * time.Millisecond
				tskEq(t, "waits", got, []time.Duration{want, want, want})
			})
		})
	})

	t.Run("Given ExponentialBackoff(10ms, 70ms)", func(t *testing.T) {
		b := monadz.ExponentialBackoff(10*time.Millisecond, 70*time.Millisecond)
		t.Run("When it is called for attempts 1 to 4 and 50", func(t *testing.T) {
			got := []time.Duration{b(1), b(2), b(3), b(4), b(50)}
			t.Run("Then the wait doubles and stops at the limit", func(t *testing.T) {
				ms := time.Millisecond
				tskEq(t, "waits", got, []time.Duration{10 * ms, 20 * ms, 40 * ms, 70 * ms, 70 * ms})
			})
		})
	})

	t.Run("Given ExponentialBackoff(1ns, the largest Duration)", func(t *testing.T) {
		limit := time.Duration(math.MaxInt64)
		b := monadz.ExponentialBackoff(time.Nanosecond, limit)
		t.Run("When it is called for attempts 62, 64, and 200", func(t *testing.T) {
			got := []time.Duration{b(62), b(64), b(200)}
			t.Run("Then the wait does not overflow and stops at the limit", func(t *testing.T) {
				tskEq(t, "waits", got, []time.Duration{1 << 61, limit, limit})
			})
		})
	})
}

func TestSleep(t *testing.T) {
	t.Run("Given a zero or negative duration", func(t *testing.T) {
		t.Run("When Sleep is called with a live ctx and a canceled ctx", func(t *testing.T) {
			c, cancel := context.WithCancel(context.Background())
			cancel()
			got := []error{
				monadz.Sleep(context.Background(), 0),
				monadz.Sleep(context.Background(), -time.Second),
				monadz.Sleep(c, 0),
			}
			t.Run("Then it returns at once with ctx.Err()", func(t *testing.T) {
				if got[0] != nil || got[1] != nil || got[2] != context.Canceled {
					t.Errorf("got %v, want [nil nil context.Canceled]", got)
				}
			})
		})
	})

	t.Run("Given a 5ms duration", func(t *testing.T) {
		t.Run("When Sleep is called", func(t *testing.T) {
			start := time.Now()
			err := monadz.Sleep(context.Background(), 5*time.Millisecond)
			elapsed := time.Since(start)
			t.Run("Then it returns nil after the wait", func(t *testing.T) {
				if err != nil || elapsed < 5*time.Millisecond {
					t.Errorf("got (%v, %v), want (nil, >= 5ms)", err, elapsed)
				}
			})
		})
	})

	t.Run("Given a 5s duration and a canceled ctx", func(t *testing.T) {
		c, cancel := context.WithCancel(context.Background())
		cancel()
		t.Run("When Sleep is called", func(t *testing.T) {
			start := time.Now()
			err := monadz.Sleep(c, 5*time.Second)
			elapsed := time.Since(start)
			t.Run("Then it returns ctx.Err() at once", func(t *testing.T) {
				if err != context.Canceled || elapsed >= time.Second {
					t.Errorf("got (%v, %v), want (context.Canceled, < 1s)", err, elapsed)
				}
			})
		})
	})
}
