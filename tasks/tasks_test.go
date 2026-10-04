package tasks_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zkrebbekx/monadz"
	"github.com/zkrebbekx/monadz/tasks"
)

var tskErrBoom = errors.New("boom")

// tskTracker counts the tasks that started, run now, and stopped, and the
// largest number that ran at the same time.
type tskTracker struct {
	started, running, maxRunning, stopped atomic.Int32
}

func (tr *tskTracker) enter() {
	tr.started.Add(1)
	n := tr.running.Add(1)
	for {
		m := tr.maxRunning.Load()
		if n <= m || tr.maxRunning.CompareAndSwap(m, n) {
			return
		}
	}
}

func (tr *tskTracker) leave() {
	tr.running.Add(-1)
	tr.stopped.Add(1)
}

// waitFor blocks until n tasks have started.
func (tr *tskTracker) waitFor(n int32) {
	for tr.started.Load() < n {
		time.Sleep(100 * time.Microsecond)
	}
}

// waitCtx is a task that waits for its ctx to end, then fails with ctx.Err().
func (tr *tskTracker) waitCtx() monadz.Task[int] {
	return monadz.NewTask(func(ctx context.Context) (int, error) {
		tr.enter()
		defer tr.leave()
		<-ctx.Done()
		return 0, ctx.Err()
	})
}

// gated is a task that waits until gate returns, then gives (v, err).
// It stops early with ctx.Err() if its ctx ends first.
func (tr *tskTracker) gated(v int, err error, gate func()) monadz.Task[int] {
	return monadz.NewTask(func(ctx context.Context) (int, error) {
		tr.enter()
		defer tr.leave()
		if gate != nil {
			g := make(chan struct{})
			go func() { gate(); close(g) }()
			select {
			case <-g:
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
		return v, err
	})
}

func (tr *tskTracker) value(v int) monadz.Task[int] { return tr.gated(v, nil, nil) }

// failAfter fails with err after n tasks of tr have started. This makes
// sure that the other tasks run when the failure happens.
func (tr *tskTracker) failAfter(n int32, err error) monadz.Task[int] {
	return tr.gated(0, err, func() { tr.waitFor(n) })
}

// panicAfter panics after n tasks of tr have started.
func (tr *tskTracker) panicAfter(n int32) monadz.Task[int] {
	return monadz.NewTask(func(context.Context) (int, error) {
		tr.enter()
		defer tr.leave()
		tr.waitFor(n)
		panic("task bug")
	})
}

// check fails the test if the tracker shows a task that still runs.
func (tr *tskTracker) checkStopped(t *testing.T) {
	t.Helper()
	if s, e := tr.started.Load(), tr.stopped.Load(); s != e {
		t.Errorf("got %d started and %d stopped tasks, want all started tasks stopped", s, e)
	}
}

func tskMustPanic(t *testing.T, f func()) (p any) {
	t.Helper()
	defer func() {
		p = recover()
		if p == nil {
			t.Fatalf("got no panic, want a panic")
		}
	}()
	f()
	return nil
}

func tskPanicError(t *testing.T, p any) {
	t.Helper()
	pe, ok := p.(*monadz.PanicError)
	if !ok {
		t.Fatalf("got panic %T(%v), want *monadz.PanicError", p, p)
	}
	if pe.Value != "task bug" {
		t.Errorf("got panic value %v, want %q", pe.Value, "task bug")
	}
	if !strings.Contains(pe.Error(), "original goroutine stack") {
		t.Errorf("got Error() %q, want the original goroutine stack", pe.Error())
	}
}

func tskEq(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %v, want %v", what, got, want)
	}
}

func tskErrIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("got error %v, want errors.Is(err, %v)", got, want)
	}
}

func TestAll(t *testing.T) {
	ctx := context.Background()

	t.Run("Given three tasks that finish in reverse order", func(t *testing.T) {
		tr := &tskTracker{}
		ts := []monadz.Task[int]{
			tr.gated(0, nil, func() { tr.waitFor(3); time.Sleep(2 * time.Millisecond) }),
			tr.gated(1, nil, func() { tr.waitFor(3); time.Sleep(time.Millisecond) }),
			tr.value(2),
		}
		t.Run("When All runs", func(t *testing.T) {
			r := tasks.All(ts...).Run(ctx)
			t.Run("Then the values keep the input order", func(t *testing.T) {
				tskEq(t, "values", r.Must(), []int{0, 1, 2})
				tskEq(t, "stopped", tr.stopped.Load(), int32(3))
			})
		})
	})

	t.Run("Given no tasks", func(t *testing.T) {
		t.Run("When All runs", func(t *testing.T) {
			r := tasks.All[int]().Run(ctx)
			t.Run("Then the result is an empty slice", func(t *testing.T) {
				if v := r.Must(); len(v) != 0 {
					t.Errorf("got %v, want []", v)
				}
			})
		})
	})

	t.Run("Given one failing task and two tasks that wait for ctx", func(t *testing.T) {
		tr := &tskTracker{}
		ts := []monadz.Task[int]{tr.waitCtx(), tr.failAfter(3, tskErrBoom), tr.waitCtx()}
		t.Run("When All runs", func(t *testing.T) {
			r := tasks.All(ts...).Run(ctx)
			stopped := tr.stopped.Load()
			t.Run("Then the failure returns and every task stopped before All returned", func(t *testing.T) {
				if r.Err() != tskErrBoom {
					t.Errorf("got %v, want %v", r.Err(), tskErrBoom)
				}
				tskEq(t, "stopped", stopped, int32(3))
			})
		})
	})

	t.Run("Given two tasks that wait for ctx", func(t *testing.T) {
		tr := &tskTracker{}
		t.Run("When the caller cancels ctx during All", func(t *testing.T) {
			c, cancel := context.WithCancel(ctx)
			go func() { tr.waitFor(2); cancel() }()
			r := tasks.All(tr.waitCtx(), tr.waitCtx()).Run(c)
			stopped := tr.stopped.Load()
			t.Run("Then All returns the ctx error after every task stopped", func(t *testing.T) {
				tskErrIs(t, r.Err(), context.Canceled)
				tskEq(t, "stopped", stopped, int32(2))
			})
		})
	})

	t.Run("Given a panicking task and two tasks that wait for ctx", func(t *testing.T) {
		tr := &tskTracker{}
		ts := []monadz.Task[int]{tr.waitCtx(), tr.panicAfter(3), tr.waitCtx()}
		t.Run("When All runs", func(t *testing.T) {
			p := tskMustPanic(t, func() { tasks.All(ts...).Run(ctx) })
			stopped := tr.stopped.Load()
			t.Run("Then the others are canceled and waited for, and the caller panics", func(t *testing.T) {
				tskPanicError(t, p)
				tskEq(t, "stopped", stopped, int32(3))
			})
		})
	})
}

func TestLimit(t *testing.T) {
	ctx := context.Background()

	t.Run("Given 20 tasks that each sleep 1ms", func(t *testing.T) {
		tr := &tskTracker{}
		ts := make([]monadz.Task[int], 20)
		for i := range ts {
			ts[i] = monadz.NewTask(func(context.Context) (int, error) {
				tr.enter()
				defer tr.leave()
				time.Sleep(time.Millisecond) // gives other tasks time to overlap
				return i * i, nil
			})
		}
		t.Run("When Limit(3) runs them", func(t *testing.T) {
			r := tasks.Limit(3, ts...).Run(ctx)
			t.Run("Then at most 3 run at once and the order stays", func(t *testing.T) {
				if m := tr.maxRunning.Load(); m < 1 || m > 3 {
					t.Errorf("got max %d running, want 1 to 3", m)
				}
				want := make([]int, 20)
				for i := range want {
					want[i] = i * i
				}
				tskEq(t, "values", r.Must(), want)
				tskEq(t, "stopped", tr.stopped.Load(), int32(20))
			})
		})
	})

	t.Run("Given 4 tasks where the second fails", func(t *testing.T) {
		tr := &tskTracker{}
		ts := []monadz.Task[int]{tr.value(1), tr.gated(0, tskErrBoom, nil), tr.value(3), tr.value(4)}
		t.Run("When Limit(1) runs them", func(t *testing.T) {
			r := tasks.Limit(1, ts...).Run(ctx)
			t.Run("Then no new task starts after the failure", func(t *testing.T) {
				if r.Err() != tskErrBoom {
					t.Errorf("got %v, want %v", r.Err(), tskErrBoom)
				}
				tskEq(t, "started", tr.started.Load(), int32(2))
				tr.checkStopped(t)
			})
		})
	})

	t.Run("Given 2 successful tasks", func(t *testing.T) {
		tr := &tskTracker{}
		for _, n := range []int{10, -1} {
			t.Run(fmt.Sprintf("When Limit(%d) runs them", n), func(t *testing.T) {
				r := tasks.Limit(n, tr.value(1), tr.value(2)).Run(ctx)
				t.Run("Then every task runs", func(t *testing.T) {
					tskEq(t, "values", r.Must(), []int{1, 2})
				})
			})
		}
	})

	t.Run("Given a task that waits for ctx, a panicking task, and 2 more", func(t *testing.T) {
		tr := &tskTracker{}
		ts := []monadz.Task[int]{tr.waitCtx(), tr.panicAfter(2), tr.value(3), tr.value(4)}
		t.Run("When Limit(2) runs them", func(t *testing.T) {
			p := tskMustPanic(t, func() { tasks.Limit(2, ts...).Run(ctx) })
			t.Run("Then no new task starts and every started task stopped", func(t *testing.T) {
				tskPanicError(t, p)
				tskEq(t, "started", tr.started.Load(), int32(2))
				tr.checkStopped(t)
			})
		})
	})
}

func TestAllSettled(t *testing.T) {
	ctx := context.Background()

	t.Run("Given a task that fails first and two that finish later", func(t *testing.T) {
		tr := &tskTracker{}
		failed := make(chan struct{})
		ts := []monadz.Task[int]{
			monadz.NewTask(func(context.Context) (int, error) { defer close(failed); return 0, tskErrBoom }),
			tr.gated(2, nil, func() { <-failed; time.Sleep(time.Millisecond) }),
			tr.gated(3, nil, func() { <-failed }),
		}
		t.Run("When AllSettled runs", func(t *testing.T) {
			r := tasks.AllSettled(ts...).Run(ctx)
			t.Run("Then every result returns and the failure did not cancel the others", func(t *testing.T) {
				rs := r.Must()
				if len(rs) != 3 {
					t.Fatalf("got %d results, want 3", len(rs))
				}
				if rs[0].Err() != tskErrBoom {
					t.Errorf("result 0: got %v, want %v", rs[0].Err(), tskErrBoom)
				}
				tskEq(t, "result 1", rs[1].Must(), 2)
				tskEq(t, "result 2", rs[2].Must(), 3)
				tr.checkStopped(t)
			})
		})
	})

	t.Run("Given a task that waits for ctx and a panicking task", func(t *testing.T) {
		tr := &tskTracker{}
		t.Run("When AllSettled runs", func(t *testing.T) {
			p := tskMustPanic(t, func() { tasks.AllSettled(tr.waitCtx(), tr.panicAfter(2)).Run(ctx) })
			t.Run("Then the other is canceled and the caller panics", func(t *testing.T) {
				tskPanicError(t, p)
				tskEq(t, "stopped", tr.stopped.Load(), int32(2))
			})
		})
	})
}

func TestAny(t *testing.T) {
	ctx := context.Background()

	t.Run("Given one success, one failure, and two tasks that wait for ctx", func(t *testing.T) {
		tr := &tskTracker{}
		ts := []monadz.Task[int]{
			tr.waitCtx(), tr.failAfter(4, tskErrBoom),
			tr.gated(7, nil, func() { tr.waitFor(4); time.Sleep(time.Millisecond) }), tr.waitCtx(),
		}
		t.Run("When Any runs", func(t *testing.T) {
			r := tasks.Any(ts...).Run(ctx)
			stopped := tr.stopped.Load()
			t.Run("Then the success wins and the rest are canceled and stopped", func(t *testing.T) {
				tskEq(t, "value", r.Must(), 7)
				tskEq(t, "stopped", stopped, int32(4))
			})
		})
	})

	t.Run("Given three tasks that fail in reverse order", func(t *testing.T) {
		tr := &tskTracker{}
		e1, e2, e3 := errors.New("e1"), errors.New("e2"), errors.New("e3")
		ts := []monadz.Task[int]{
			tr.gated(0, e1, func() { tr.waitFor(3); time.Sleep(2 * time.Millisecond) }),
			tr.gated(0, e2, func() { tr.waitFor(3); time.Sleep(time.Millisecond) }),
			tr.gated(0, e3, nil),
		}
		t.Run("When Any runs", func(t *testing.T) {
			r := tasks.Any(ts...).Run(ctx)
			t.Run("Then the error joins all errors in input order", func(t *testing.T) {
				tskEq(t, "message", r.Err().Error(), "e1\ne2\ne3")
				tskErrIs(t, r.Err(), e2)
			})
		})
	})

	t.Run("Given no tasks", func(t *testing.T) {
		t.Run("When Any runs", func(t *testing.T) {
			r := tasks.Any[int]().Run(ctx)
			t.Run("Then it returns an error", func(t *testing.T) {
				if r.IsOk() || !strings.Contains(r.Err().Error(), "no tasks") {
					t.Errorf("got %v, want an error about no tasks", r)
				}
			})
		})
	})

	t.Run("Given a task that waits for ctx and a panicking task", func(t *testing.T) {
		tr := &tskTracker{}
		t.Run("When Any runs", func(t *testing.T) {
			p := tskMustPanic(t, func() { tasks.Any(tr.waitCtx(), tr.panicAfter(2)).Run(ctx) })
			t.Run("Then the caller panics after the other stopped", func(t *testing.T) {
				tskPanicError(t, p)
				tskEq(t, "stopped", tr.stopped.Load(), int32(2))
			})
		})
	})
}

func TestRace(t *testing.T) {
	ctx := context.Background()

	t.Run("Given a failure that stops first and two tasks that wait for ctx", func(t *testing.T) {
		tr := &tskTracker{}
		ts := []monadz.Task[int]{tr.waitCtx(), tr.failAfter(3, tskErrBoom), tr.waitCtx()}
		t.Run("When Race runs", func(t *testing.T) {
			r := tasks.Race(ts...).Run(ctx)
			stopped := tr.stopped.Load()
			t.Run("Then the failure wins and the others are canceled and stopped", func(t *testing.T) {
				if r.Err() != tskErrBoom {
					t.Errorf("got %v, want %v", r.Err(), tskErrBoom)
				}
				tskEq(t, "stopped", stopped, int32(3))
			})
		})
	})

	t.Run("Given a success that stops first and a task that waits for ctx", func(t *testing.T) {
		tr := &tskTracker{}
		t.Run("When Race runs", func(t *testing.T) {
			r := tasks.Race(tr.waitCtx(), tr.gated(5, nil, func() { tr.waitFor(2) })).Run(ctx)
			t.Run("Then the success wins", func(t *testing.T) {
				tskEq(t, "value", r.Must(), 5)
				tskEq(t, "stopped", tr.stopped.Load(), int32(2))
			})
		})
	})

	t.Run("Given no tasks", func(t *testing.T) {
		t.Run("When Race runs", func(t *testing.T) {
			r := tasks.Race[int]().Run(ctx)
			t.Run("Then it returns an error", func(t *testing.T) {
				if r.IsOk() || !strings.Contains(r.Err().Error(), "no tasks") {
					t.Errorf("got %v, want an error about no tasks", r)
				}
			})
		})
	})

	t.Run("Given a task that waits for ctx and a panicking task", func(t *testing.T) {
		tr := &tskTracker{}
		t.Run("When Race runs", func(t *testing.T) {
			p := tskMustPanic(t, func() { tasks.Race(tr.waitCtx(), tr.panicAfter(2)).Run(ctx) })
			t.Run("Then the caller panics after the other stopped", func(t *testing.T) {
				tskPanicError(t, p)
				tskEq(t, "stopped", tr.stopped.Load(), int32(2))
			})
		})
	})
}

func TestTraverse(t *testing.T) {
	ctx := context.Background()
	items := []int{1, 2, 3, 4, 5, 6, 7, 8}

	t.Run("Given 8 items and a function that succeeds", func(t *testing.T) {
		tr := &tskTracker{}
		f := func(ctx context.Context, v int) (string, error) {
			tr.enter()
			defer tr.leave()
			time.Sleep(time.Millisecond)
			return fmt.Sprint(v * 10), nil
		}
		t.Run("When Traverse runs with a limit of 2", func(t *testing.T) {
			r := tasks.Traverse(slices.Values(items), 2, f).Run(ctx)
			t.Run("Then the order stays and at most 2 run at once", func(t *testing.T) {
				tskEq(t, "values", r.Must(), []string{"10", "20", "30", "40", "50", "60", "70", "80"})
				if m := tr.maxRunning.Load(); m > 2 {
					t.Errorf("got max %d running, want <= 2", m)
				}
			})
		})
	})

	t.Run("Given 8 items and a function that fails on item 2", func(t *testing.T) {
		tr := &tskTracker{}
		f := func(ctx context.Context, v int) (int, error) {
			tr.enter()
			defer tr.leave()
			if v == 2 {
				return 0, tskErrBoom
			}
			return v, nil
		}
		t.Run("When Traverse runs with a limit of 1", func(t *testing.T) {
			r := tasks.Traverse(slices.Values(items), 1, f).Run(ctx)
			t.Run("Then it stops at the failure and starts no new call", func(t *testing.T) {
				if r.Err() != tskErrBoom {
					t.Errorf("got %v, want %v", r.Err(), tskErrBoom)
				}
				tskEq(t, "started", tr.started.Load(), int32(2))
			})
		})
	})

	t.Run("Given 8 items and a function that fails on item 5 and waits for ctx on the others", func(t *testing.T) {
		tr := &tskTracker{}
		f := func(ctx context.Context, v int) (int, error) {
			tr.enter()
			defer tr.leave()
			if v == 5 {
				tr.waitFor(8)
				return 0, tskErrBoom
			}
			<-ctx.Done()
			return 0, ctx.Err()
		}
		t.Run("When Traverse runs with no limit", func(t *testing.T) {
			r := tasks.Traverse(slices.Values(items), 0, f).Run(ctx)
			stopped := tr.stopped.Load()
			t.Run("Then the calls in progress are canceled and the failure returns", func(t *testing.T) {
				if r.Err() != tskErrBoom {
					t.Errorf("got %v, want %v", r.Err(), tskErrBoom)
				}
				tskEq(t, "stopped", stopped, int32(8))
			})
		})
	})
}

func TestZip(t *testing.T) {
	ctx := context.Background()

	t.Run("Given two successful tasks", func(t *testing.T) {
		t.Run("When Zip runs", func(t *testing.T) {
			r := tasks.Zip(monadz.TaskOf(1), monadz.TaskOf("a")).Run(ctx)
			t.Run("Then it returns a Pair", func(t *testing.T) {
				tskEq(t, "pair", r.Must(), monadz.PairOf(1, "a"))
			})
		})
	})

	t.Run("Given a task that waits for ctx and a failing task", func(t *testing.T) {
		tr := &tskTracker{}
		failing := monadz.NewTask(func(context.Context) (string, error) { tr.waitFor(1); return "", tskErrBoom })
		t.Run("When Zip runs", func(t *testing.T) {
			r := tasks.Zip(tr.waitCtx(), failing).Run(ctx)
			stopped := tr.stopped.Load()
			t.Run("Then the other is canceled and the failure returns", func(t *testing.T) {
				if r.Err() != tskErrBoom {
					t.Errorf("got %v, want %v", r.Err(), tskErrBoom)
				}
				tskEq(t, "stopped", stopped, int32(1))
			})
		})
	})

	t.Run("Given three successful tasks", func(t *testing.T) {
		t.Run("When Zip3 runs", func(t *testing.T) {
			r := tasks.Zip3(monadz.TaskOf(1), monadz.TaskOf("a"), monadz.TaskOf(true)).Run(ctx)
			t.Run("Then it returns a Triple", func(t *testing.T) {
				tskEq(t, "triple", r.Must(), monadz.TripleOf(1, "a", true))
			})
		})
	})

	t.Run("Given two tasks that wait for ctx and a third that fails", func(t *testing.T) {
		tr := &tskTracker{}
		failing := monadz.NewTask(func(context.Context) (bool, error) { tr.waitFor(2); return false, tskErrBoom })
		t.Run("When Zip3 runs", func(t *testing.T) {
			r := tasks.Zip3(tr.waitCtx(), tr.waitCtx(), failing).Run(ctx)
			stopped := tr.stopped.Load()
			t.Run("Then the first two are canceled and the failure returns", func(t *testing.T) {
				if r.Err() != tskErrBoom {
					t.Errorf("got %v, want %v", r.Err(), tskErrBoom)
				}
				tskEq(t, "stopped", stopped, int32(2))
			})
		})
	})

	t.Run("Given a task that waits for ctx, a panicking task, and a success", func(t *testing.T) {
		tr := &tskTracker{}
		t.Run("When Zip3 runs", func(t *testing.T) {
			p := tskMustPanic(t, func() { tasks.Zip3(tr.waitCtx(), tr.panicAfter(2), monadz.TaskOf(true)).Run(ctx) })
			stopped := tr.stopped.Load()
			t.Run("Then the caller panics after the others stopped", func(t *testing.T) {
				tskPanicError(t, p)
				tskEq(t, "stopped", stopped, int32(2))
			})
		})
	})
}
