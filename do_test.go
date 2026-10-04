package monadz_test

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/zkrebbekx/monadz"
)

func TestDo(t *testing.T) {
	t.Run("Given a Do block", func(t *testing.T) {
		t.Run("When every Try succeeds", func(t *testing.T) {
			r := monadz.Do(func() int {
				a := monadz.Of(strconv.Atoi("2")).Try()
				b := monadz.Some(3).Try()
				return a * b
			})
			t.Run("Then Do returns Ok of the value", func(t *testing.T) {
				wantEq(t, r.Must(), 6)
			})
		})
		t.Run("When a Result.Try fails", func(t *testing.T) {
			after := false
			r := monadz.Do(func() int {
				_ = monadz.Err[int](resErrBoom).Try()
				after = true
				return 1
			})
			t.Run("Then Do returns the error and the code after Try does not run", func(t *testing.T) {
				wantEq(t, r.Err(), resErrBoom)
				wantEq(t, after, false)
			})
		})
		t.Run("When an Option.Try gets None", func(t *testing.T) {
			after := false
			r := monadz.Do(func() int {
				_ = monadz.None[int]().Try()
				after = true
				return 1
			})
			t.Run("Then Do returns ErrNone and the code after Try does not run", func(t *testing.T) {
				wantEq(t, errors.Is(r.Err(), monadz.ErrNone), true)
				wantEq(t, after, false)
			})
		})
		t.Run("When f writes to a nil map", func(t *testing.T) {
			f := func() {
				monadz.Do(func() int {
					var m map[string]int
					m["a"] = 1
					return 0
				})
			}
			t.Run("Then Do does not recover the panic", func(t *testing.T) {
				mustPanic(t, f)
			})
		})
		t.Run("When f calls panic(\"x\")", func(t *testing.T) {
			t.Run("Then Do passes the same panic value through", func(t *testing.T) {
				wantPanic(t, func() { monadz.Do(func() int { panic("x") }) }, "x")
			})
		})
		t.Run("When Do blocks are nested", func(t *testing.T) {
			innerAfter := false
			outer := monadz.Do(func() int {
				inner := monadz.Do(func() int {
					_ = monadz.Err[int](resErrBoom).Try()
					innerAfter = true
					return 0
				})
				wantEq(t, inner.Err(), resErrBoom)
				return inner.ValueOr(10) + 1
			})
			t.Run("Then the inner failure stops only the inner block", func(t *testing.T) {
				wantEq(t, innerAfter, false)
				wantEq(t, outer.Must(), 11)
			})
		})
		t.Run("When an inner Do succeeds and the outer Try fails", func(t *testing.T) {
			other := errors.New("other")
			outer := monadz.Do(func() int {
				inner := monadz.Do(func() int { return 5 })
				_ = monadz.Err[int](other).Try()
				return inner.Must()
			})
			t.Run("Then the outer Do returns the outer error", func(t *testing.T) {
				wantEq(t, outer.Err(), other)
			})
		})
	})
}

func TestDoRecover(t *testing.T) {
	t.Run("Given a DoRecover block", func(t *testing.T) {
		t.Run("When every Try succeeds", func(t *testing.T) {
			r := monadz.DoRecover(func() string { return monadz.Ok("a").Try() + monadz.Some("b").Try() })
			t.Run("Then DoRecover returns Ok of the value", func(t *testing.T) {
				wantEq(t, r.Must(), "ab")
			})
		})
		t.Run("When a Result.Try fails", func(t *testing.T) {
			r := monadz.DoRecover(func() int { return monadz.Err[int](resErrBoom).Try() })
			t.Run("Then DoRecover returns the error", func(t *testing.T) {
				wantEq(t, r.Err(), resErrBoom)
			})
		})
		t.Run("When an Option.Try gets None", func(t *testing.T) {
			r := monadz.DoRecover(func() int { return monadz.None[int]().Try() })
			t.Run("Then DoRecover returns ErrNone", func(t *testing.T) {
				wantEq(t, errors.Is(r.Err(), monadz.ErrNone), true)
			})
		})
		t.Run("When f writes to a nil map", func(t *testing.T) {
			r := monadz.DoRecover(func() int {
				var m map[string]int
				m["a"] = 1
				return 0
			})
			t.Run("Then DoRecover returns a *PanicError that unwraps to a runtime.Error", func(t *testing.T) {
				var pe *monadz.PanicError
				wantEq(t, errors.As(r.Err(), &pe), true)
				wantNotEmpty(t, pe.Stack)
				var re runtime.Error
				wantEq(t, errors.As(r.Err(), &re), true)
				wantEq(t, pe.Unwrap(), error(re))
				wantPrefix(t, pe.Error(), "panic: assignment to entry in nil map")
				wantNotContains(t, pe.Error(), "original goroutine stack")
			})
		})
		t.Run("When f reads a slice out of range", func(t *testing.T) {
			r := monadz.DoRecover(func() int {
				s := []int{}
				i := 3
				return s[i]
			})
			t.Run("Then errors.As finds the runtime.Error", func(t *testing.T) {
				var re runtime.Error
				wantEq(t, errors.As(r.Err(), &re), true)
			})
		})
		t.Run("When f calls panic(\"x\")", func(t *testing.T) {
			r := monadz.DoRecover(func() int { panic("x") })
			t.Run("Then the *PanicError holds \"x\", a stack, and Unwrap gives nil", func(t *testing.T) {
				var pe *monadz.PanicError
				wantEq(t, errors.As(r.Err(), &pe), true)
				wantEq(t, pe.Value, "x")
				wantNotEmpty(t, pe.Stack)
				wantNil(t, pe.Unwrap())
				wantEq(t, pe.Error(), "panic: x")
			})
		})
		t.Run("When f panics with an error value", func(t *testing.T) {
			r := monadz.DoRecover(func() int { panic(resErrBoom) })
			t.Run("Then errors.Is finds the error through the *PanicError", func(t *testing.T) {
				wantEq(t, errors.Is(r.Err(), resErrBoom), true)
			})
		})
	})
}

func TestDoOption(t *testing.T) {
	t.Run("Given a DoOption block", func(t *testing.T) {
		t.Run("When every Option.Try succeeds", func(t *testing.T) {
			o := monadz.DoOption(func() int { return monadz.Some(2).Try() + 1 })
			t.Run("Then DoOption returns Some of the value", func(t *testing.T) {
				wantEq(t, o.Must(), 3)
			})
		})
		t.Run("When an Option.Try gets None", func(t *testing.T) {
			after := false
			o := monadz.DoOption(func() int {
				_ = monadz.None[int]().Try()
				after = true
				return 1
			})
			t.Run("Then DoOption returns None and the code after Try does not run", func(t *testing.T) {
				wantEq(t, o.IsNone(), true)
				wantEq(t, after, false)
			})
		})
		t.Run("When a Result.Try fails inside DoOption", func(t *testing.T) {
			f := func() { monadz.DoOption(func() int { return monadz.Err[int](resErrBoom).Try() }) }
			t.Run("Then DoOption panics", func(t *testing.T) {
				mustPanic(t, f)
			})
		})
		t.Run("When f calls panic(\"x\")", func(t *testing.T) {
			t.Run("Then DoOption passes the panic through", func(t *testing.T) {
				wantPanic(t, func() { monadz.DoOption(func() int { panic("x") }) }, "x")
			})
		})
	})
}

func TestTryOutsideDo(t *testing.T) {
	t.Run("Given a Try call outside any Do", func(t *testing.T) {
		t.Run("When Result.Try gets a failure", func(t *testing.T) {
			p := recovered(func() { monadz.Err[int](resErrBoom).Try() })
			t.Run("Then it panics with a message that names monadz.Do", func(t *testing.T) {
				err, ok := p.(error)
				wantEq(t, ok, true)
				wantContains(t, err.Error(), "outside monadz.Do")
				wantContains(t, err.Error(), "boom")
			})
		})
		t.Run("When Option.Try gets None", func(t *testing.T) {
			p := recovered(func() { monadz.None[int]().Try() })
			t.Run("Then it panics with a message that names monadz.Do", func(t *testing.T) {
				err, ok := p.(error)
				wantEq(t, ok, true)
				wantContains(t, err.Error(), "outside monadz.Do")
			})
		})
		t.Run("When Try gets a success", func(t *testing.T) {
			t.Run("Then it returns the value without a panic", func(t *testing.T) {
				wantEq(t, monadz.Ok(1).Try(), 1)
				wantEq(t, strings.Repeat(monadz.Some("a").Try(), 2), "aa")
			})
		})
	})
}
