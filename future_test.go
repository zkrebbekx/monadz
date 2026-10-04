package lift_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zkrebbekx/lift"
)

func TestFuture(t *testing.T) {
	ctx := context.Background()

	t.Run("Given a Task that blocks until the test releases it", func(t *testing.T) {
		release := make(chan struct{})
		started := make(chan struct{})
		tk := lift.NewTask(func(context.Context) (int, error) {
			close(started)
			<-release
			return 4, nil
		})
		t.Run("When it is started, polled, released, and polled again", func(t *testing.T) {
			f := tk.Start(ctx)
			<-started // Start returned while the work runs: it runs in another goroutine
			before := f.Poll()
			doneBefore := tskClosed(f.Done())
			close(release)
			<-f.Done()
			after := f.Poll()
			t.Run("Then Poll is None before and Some after, and Await returns the value", func(t *testing.T) {
				if doneBefore {
					t.Error("got Done closed before the release, want open")
				}
				if !before.IsNone() {
					t.Errorf("got Poll %v before the release, want None", before)
				}
				if !after.IsSome() {
					t.Fatalf("got Poll %v after Done, want Some", after)
				}
				tskEq(t, "Poll value", after.Must().Must(), 4)
				tskEq(t, "Await value", f.Await().Must(), 4)
			})
		})
	})

	t.Run("Given a started Task that waits for its ctx", func(t *testing.T) {
		started, stopped := make(chan struct{}), make(chan struct{})
		f := tskWaitCtx(started, stopped).Start(ctx)
		<-started
		t.Run("When Cancel is called", func(t *testing.T) {
			f.Cancel()
			r := f.Await()
			t.Run("Then the Task's ctx is canceled", func(t *testing.T) {
				tskIs(t, r.Err(), context.Canceled)
				if !tskClosed(stopped) {
					t.Error("got a running Task, want it stopped")
				}
			})
		})
	})

	t.Run("Given a started Task that fails", func(t *testing.T) {
		t.Run("When Await is called", func(t *testing.T) {
			r := lift.TaskErr[int](tskErrBoom).Start(ctx).Await()
			t.Run("Then it returns the error", func(t *testing.T) {
				if r.Err() != tskErrBoom {
					t.Errorf("got %v, want %v", r.Err(), tskErrBoom)
				}
			})
		})
	})

	t.Run("Given a started Task that panics with an error", func(t *testing.T) {
		f := lift.NewTask(func(context.Context) (int, error) { panic(tskErrBoom) }).Start(ctx)
		<-f.Done()
		t.Run("When Await and Poll are called", func(t *testing.T) {
			p := tskMustPanic(t, func() { f.Await() })
			pp := tskMustPanic(t, func() { f.Poll() })
			t.Run("Then both re-panic a *PanicError with the original stack", func(t *testing.T) {
				tskPanicError(t, p, error(tskErrBoom))
				tskPanicError(t, pp, error(tskErrBoom))
				pe := p.(*lift.PanicError)
				if !errors.Is(pe, tskErrBoom) {
					t.Error("got errors.Is false, want the panic error reachable")
				}
				if len(pe.Stack) == 0 {
					t.Error("got an empty stack, want the original stack")
				}
			})
		})
	})

	t.Run("Given a started Task that waits for its ctx", func(t *testing.T) {
		stopped := make(chan struct{})
		f := tskWaitCtx(nil, stopped).Start(ctx)
		t.Run("When AwaitContext gets a ctx that ends after 10ms", func(t *testing.T) {
			c, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
			defer cancel()
			r := f.AwaitContext(c)
			stillRuns := !tskClosed(f.Done())
			f.Cancel()
			<-stopped
			t.Run("Then it returns ctx.Err() and does not cancel the Task", func(t *testing.T) {
				tskIs(t, r.Err(), context.DeadlineExceeded)
				if !stillRuns {
					t.Error("got the Task stopped, want it still running")
				}
			})
		})
	})

	t.Run("Given a started Task that succeeds", func(t *testing.T) {
		t.Run("When AwaitContext is called", func(t *testing.T) {
			r := lift.TaskOf(6).Start(ctx).AwaitContext(ctx)
			t.Run("Then it returns the result", func(t *testing.T) { tskEq(t, "value", r.Must(), 6) })
		})
	})

	t.Run("Given a canceled ctx", func(t *testing.T) {
		c, cancel := context.WithCancel(ctx)
		cancel()
		t.Run("When Start is called", func(t *testing.T) {
			ran := false
			r := lift.NewTask(func(context.Context) (int, error) { ran = true; return 1, nil }).Start(c).Await()
			t.Run("Then the work does not run", func(t *testing.T) {
				if ran {
					t.Error("got the work run, want it not started")
				}
				tskIs(t, r.Err(), context.Canceled)
			})
		})
	})
}
