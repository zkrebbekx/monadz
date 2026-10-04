package monadz_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zkrebbekx/monadz"
)

var tskErrBoom = errors.New("boom")

// tskMustPanic calls f and returns the value that f panicked with. It fails
// the test if f does not panic.
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

// tskPanicError asserts that p is a *monadz.PanicError that crossed
// goroutines with the value want.
func tskPanicError(t *testing.T, p any, want any) {
	t.Helper()
	pe, ok := p.(*monadz.PanicError)
	if !ok {
		t.Fatalf("got panic %T(%v), want *monadz.PanicError", p, p)
	}
	if pe.Value != want {
		t.Errorf("got panic value %v, want %v", pe.Value, want)
	}
	if !strings.Contains(pe.Error(), "original goroutine stack") {
		t.Errorf("got Error() %q, want it to contain the original goroutine stack", pe.Error())
	}
}

// tskEq fails the test if got and want are not deeply equal.
func tskEq(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %v, want %v", what, got, want)
	}
}

// tskIs fails the test if errors.Is(err, target) is false.
func tskIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("got error %v, want errors.Is(err, %v)", err, target)
	}
}

// tskWaitCtx returns a Task that waits until its ctx ends. It closes
// started (if not nil) when it begins and stopped (if not nil) when it
// returns.
func tskWaitCtx(started, stopped chan struct{}) monadz.Task[int] {
	return monadz.NewTask(func(ctx context.Context) (int, error) {
		if started != nil {
			close(started)
		}
		if stopped != nil {
			defer close(stopped)
		}
		<-ctx.Done()
		return 0, ctx.Err()
	})
}

// tskClosed reports whether ch is closed, without a wait.
func tskClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestTaskConstructors(t *testing.T) {
	ctx := context.Background()
	t.Run("Given NewTask with a Go-shaped function", func(t *testing.T) {
		t.Run("When Run is called", func(t *testing.T) {
			var got context.Context
			r := monadz.NewTask(func(c context.Context) (int, error) { got = c; return 7, nil }).Run(ctx)
			t.Run("Then it returns the value and passes the ctx", func(t *testing.T) {
				tskEq(t, "value", r.Must(), 7)
				if got != ctx {
					t.Errorf("got ctx %v, want %v", got, ctx)
				}
			})
		})
		t.Run("When the function fails", func(t *testing.T) {
			r := monadz.NewTask(func(context.Context) (int, error) { return 0, tskErrBoom }).Run(ctx)
			t.Run("Then Run returns the error", func(t *testing.T) {
				if r.Err() != tskErrBoom {
					t.Errorf("got %v, want %v", r.Err(), tskErrBoom)
				}
			})
		})
		t.Run("When the function panics", func(t *testing.T) {
			p := tskMustPanic(t, func() {
				monadz.NewTask(func(context.Context) (int, error) { panic("bug") }).Run(ctx)
			})
			t.Run("Then the panic reaches the caller unchanged", func(t *testing.T) {
				tskEq(t, "panic", p, "bug")
			})
		})
	})

	t.Run("Given TaskResult, TaskOf, and TaskErr", func(t *testing.T) {
		t.Run("When each runs", func(t *testing.T) {
			res := monadz.TaskResult(func(context.Context) monadz.Result[string] { return monadz.Ok("hi") }).Run(ctx)
			ok := monadz.TaskOf(3).Run(ctx)
			bad := monadz.TaskErr[int](tskErrBoom).Run(ctx)
			nilErr := monadz.TaskErr[int](nil).Run(ctx)
			t.Run("Then each gives its value or error", func(t *testing.T) {
				tskEq(t, "TaskResult", res.Must(), "hi")
				tskEq(t, "TaskOf", ok.Must(), 3)
				if bad.Err() != tskErrBoom {
					t.Errorf("TaskErr: got %v, want %v", bad.Err(), tskErrBoom)
				}
				if nilErr.Err() != monadz.ErrNilError {
					t.Errorf("TaskErr(nil): got %v, want ErrNilError", nilErr.Err())
				}
			})
		})
	})

	t.Run("Given a zero Task", func(t *testing.T) {
		t.Run("When Run is called", func(t *testing.T) {
			var tk monadz.Task[int]
			r := tk.Run(ctx)
			t.Run("Then it returns ErrZeroTask", func(t *testing.T) {
				tskIs(t, r.Err(), monadz.ErrZeroTask)
			})
		})
	})

	t.Run("Given a canceled ctx", func(t *testing.T) {
		c, cancel := context.WithCancel(ctx)
		cancel()
		t.Run("When Run is called", func(t *testing.T) {
			var calls atomic.Int32
			r := monadz.NewTask(func(context.Context) (int, error) { calls.Add(1); return 1, nil }).Run(c)
			t.Run("Then the work does not start and the error is ctx.Err()", func(t *testing.T) {
				tskEq(t, "calls", calls.Load(), int32(0))
				tskIs(t, r.Err(), context.Canceled)
			})
		})
	})

	t.Run("Given a Task that counts its runs", func(t *testing.T) {
		var calls atomic.Int32
		tk := monadz.NewTask(func(context.Context) (int32, error) { return calls.Add(1), nil })
		before := calls.Load()
		t.Run("When it runs twice", func(t *testing.T) {
			first, second := tk.Run(ctx).Must(), tk.Run(ctx).Must()
			t.Run("Then nothing ran at construction and each Run runs again", func(t *testing.T) {
				tskEq(t, "before", before, int32(0))
				tskEq(t, "first", first, int32(1))
				tskEq(t, "second", second, int32(2))
			})
		})
	})
}

func TestTaskTransformations(t *testing.T) {
	ctx := context.Background()
	good := monadz.TaskOf(2)
	bad := monadz.TaskErr[int](tskErrBoom)

	t.Run("Given a successful Task and a failing Task", func(t *testing.T) {
		t.Run("When Map is applied", func(t *testing.T) {
			g := good.Map(func(v int) string { return strings.Repeat("x", v) }).Run(ctx)
			b := bad.Map(func(int) string { panic("must not run") }).Run(ctx)
			t.Run("Then it changes the value and keeps the error", func(t *testing.T) {
				tskEq(t, "value", g.Must(), "xx")
				if b.Err() != tskErrBoom {
					t.Errorf("got %v, want %v", b.Err(), tskErrBoom)
				}
			})
		})

		t.Run("When Then is applied", func(t *testing.T) {
			type key struct{}
			c := context.WithValue(ctx, key{}, "v")
			var seen any
			g := good.Then(func(c context.Context, v int) (string, error) {
				seen = c.Value(key{})
				return "n" + strings.Repeat("!", v), nil
			}).Run(c)
			f := good.Then(func(context.Context, int) (string, error) { return "", tskErrBoom }).Run(ctx)
			b := bad.Then(func(context.Context, int) (string, error) { panic("must not run") }).Run(ctx)
			t.Run("Then the step gets the ctx and errors pass through", func(t *testing.T) {
				tskEq(t, "value", g.Must(), "n!!")
				tskEq(t, "ctx value", seen, "v")
				if f.Err() != tskErrBoom || b.Err() != tskErrBoom {
					t.Errorf("got %v and %v, want %v", f.Err(), b.Err(), tskErrBoom)
				}
			})
		})

		t.Run("When AndThen is applied", func(t *testing.T) {
			g := good.AndThen(func(v int) monadz.Task[int] { return monadz.TaskOf(v * 10) }).Run(ctx)
			b := bad.AndThen(func(int) monadz.Task[int] { panic("must not run") }).Run(ctx)
			t.Run("Then it runs the next Task only after a success", func(t *testing.T) {
				tskEq(t, "value", g.Must(), 20)
				if b.Err() != tskErrBoom {
					t.Errorf("got %v, want %v", b.Err(), tskErrBoom)
				}
			})
		})

		t.Run("When MapErr and Wrap are applied", func(t *testing.T) {
			other := errors.New("other")
			m := bad.MapErr(func(error) error { return other }).Run(ctx)
			w := bad.Wrap("load").Run(ctx)
			g := good.Wrap("load").Run(ctx)
			t.Run("Then the error changes and the value stays", func(t *testing.T) {
				if m.Err() != other {
					t.Errorf("MapErr: got %v, want %v", m.Err(), other)
				}
				tskEq(t, "Wrap message", w.Err().Error(), "load: boom")
				tskIs(t, w.Err(), tskErrBoom)
				tskEq(t, "Wrap value", g.Must(), 2)
			})
		})

		t.Run("When Inspect and InspectErr are applied", func(t *testing.T) {
			var v int
			var e error
			good.Inspect(func(x int) { v = x }).InspectErr(func(error) { panic("no") }).Run(ctx)
			bad.Inspect(func(int) { panic("no") }).InspectErr(func(err error) { e = err }).Run(ctx)
			t.Run("Then each callback sees only its own case", func(t *testing.T) {
				tskEq(t, "Inspect", v, 2)
				if e != tskErrBoom {
					t.Errorf("InspectErr: got %v, want %v", e, tskErrBoom)
				}
			})
		})

		t.Run("When Recover and OrElse are applied", func(t *testing.T) {
			r := bad.Recover(func(error) (int, error) { return 9, nil }).Run(ctx)
			o := bad.OrElse(func(error) monadz.Task[int] { return monadz.TaskOf(8) }).Run(ctx)
			og := good.OrElse(func(error) monadz.Task[int] { panic("must not run") }).Run(ctx)
			t.Run("Then a failure is replaced and a success stays", func(t *testing.T) {
				tskEq(t, "Recover", r.Must(), 9)
				tskEq(t, "OrElse", o.Must(), 8)
				tskEq(t, "OrElse on success", og.Must(), 2)
			})
		})
	})
}

func TestTaskZipWith(t *testing.T) {
	ctx := context.Background()
	sum := func(a, b int) int { return a + b }

	t.Run("Given two Tasks that pass only when both wait at a barrier", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(2)
		barrier := func(v int) monadz.Task[int] {
			return monadz.NewTask(func(ctx context.Context) (int, error) {
				wg.Done()
				wg.Wait() // passes only when the other Task also runs
				return v, nil
			})
		}
		t.Run("When ZipWith runs them", func(t *testing.T) {
			done := make(chan monadz.Result[int], 1)
			go func() {
				done <- barrier(2).ZipWith(barrier(3), func(a, b int) int { return a * b }).Run(ctx)
			}()
			t.Run("Then they run at the same time and the values combine", func(t *testing.T) {
				select {
				case r := <-done:
					tskEq(t, "value", r.Must(), 6)
				case <-time.After(5 * time.Second):
					t.Fatal("got a deadlock, want both Tasks to run at the same time")
				}
			})
		})
	})

	t.Run("Given a failing Task and a Task that waits for ctx", func(t *testing.T) {
		for _, failFirst := range []bool{true, false} {
			name := "When the failing Task is the first argument"
			if !failFirst {
				name = "When the failing Task is the second argument"
			}
			t.Run(name, func(t *testing.T) {
				started, stopped := make(chan struct{}), make(chan struct{})
				// The failing Task fails only after the other Task runs.
				failing := monadz.NewTask(func(context.Context) (int, error) { <-started; return 0, tskErrBoom })
				var tk monadz.Task[int]
				if failFirst {
					tk = failing.ZipWith(tskWaitCtx(started, stopped), sum)
				} else {
					tk = tskWaitCtx(started, stopped).ZipWith(failing, sum)
				}
				r := tk.Run(ctx)
				closed := tskClosed(stopped)
				t.Run("Then the other is canceled and stopped, and the failure returns", func(t *testing.T) {
					if r.Err() != tskErrBoom {
						t.Errorf("got %v, want %v", r.Err(), tskErrBoom)
					}
					if !closed {
						t.Error("got a running Task after ZipWith returned, want it stopped")
					}
				})
			})
		}
	})

	t.Run("Given a panicking Task and a Task that waits for ctx", func(t *testing.T) {
		for _, first := range []bool{true, false} {
			name := "When the panicking Task is the first argument"
			if !first {
				name = "When the panicking Task is the second argument"
			}
			t.Run(name, func(t *testing.T) {
				started, stopped := make(chan struct{}), make(chan struct{})
				// The panicking Task panics only after the other Task runs.
				panicky := monadz.NewTask(func(context.Context) (int, error) { <-started; panic("zip bug") })
				var tk monadz.Task[int]
				if first {
					tk = panicky.ZipWith(tskWaitCtx(started, stopped), sum)
				} else {
					tk = tskWaitCtx(started, stopped).ZipWith(panicky, sum)
				}
				p := tskMustPanic(t, func() { tk.Run(ctx) })
				closed := tskClosed(stopped)
				t.Run("Then the caller gets a *PanicError after the other stopped", func(t *testing.T) {
					tskPanicError(t, p, "zip bug")
					if !closed {
						t.Error("got a running Task after the panic, want it stopped")
					}
				})
			})
		}
	})

	t.Run("Given a ZipWith nested in a ZipWith with a panic inside", func(t *testing.T) {
		inner := monadz.NewTask(func(context.Context) (int, error) { panic("deep") }).ZipWith(monadz.TaskOf(1), sum)
		t.Run("When the outer Task runs", func(t *testing.T) {
			p := tskMustPanic(t, func() { inner.ZipWith(monadz.TaskOf(1), sum).Run(ctx) })
			t.Run("Then the original panic value reaches the caller", func(t *testing.T) {
				tskPanicError(t, p, "deep")
			})
		})
	})
}
