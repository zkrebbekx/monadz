package monadz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/zkrebbekx/monadz"
)

func TestErrorValues(t *testing.T) {
	t.Run("Given the sentinel errors", func(t *testing.T) {
		t.Run("When their messages are read", func(t *testing.T) {
			t.Run("Then they are distinct and name monadz", func(t *testing.T) {
				resEq(t, monadz.ErrNone.Error(), "monadz: value is None")
				resEq(t, monadz.ErrNilError.Error(), "monadz: failure constructed with a nil error")
				resEq(t, errors.Is(monadz.ErrNone, monadz.ErrNilError), false)
			})
		})
	})
}

func TestPanicError(t *testing.T) {
	t.Run("Given a *PanicError", func(t *testing.T) {
		t.Run("When it holds an error value", func(t *testing.T) {
			pe := &monadz.PanicError{Value: resErrBoom}
			t.Run("Then Error formats the value and Unwrap returns it", func(t *testing.T) {
				resEq(t, pe.Error(), "panic: boom")
				resEq(t, pe.Unwrap(), resErrBoom)
				resEq(t, errors.Is(pe, resErrBoom), true)
			})
		})
		t.Run("When it holds a non-error value", func(t *testing.T) {
			pe := &monadz.PanicError{Value: 42}
			t.Run("Then Error formats the value and Unwrap returns nil", func(t *testing.T) {
				resEq(t, pe.Error(), "panic: 42")
				resNil(t, pe.Unwrap())
			})
		})
		t.Run("When a Task panics and Await rethrows the panic", func(t *testing.T) {
			fut := monadz.NewTask(func(context.Context) (int, error) { panic("x") }).Start(context.Background())
			p := resRecover(func() { fut.Await() })
			t.Run("Then the rethrown *PanicError includes the original stack in Error", func(t *testing.T) {
				pe, ok := p.(*monadz.PanicError)
				resEq(t, ok, true)
				resEq(t, pe.Value, "x")
				resNotEmpty(t, pe.Stack)
				msg := pe.Error()
				resPrefix(t, msg, "panic: x\n\noriginal goroutine stack:\n")
				resContains(t, msg, string(pe.Stack))
			})
		})
	})
}
