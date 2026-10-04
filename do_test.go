package lift_test

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/zkrebbekx/lift"
)

func TestDo(t *testing.T) {
	t.Run("Given a Do block", func(t *testing.T) {
		t.Run("When every Try succeeds", func(t *testing.T) {
			r := lift.Do(func() int {
				a := lift.Of(strconv.Atoi("2")).Try()
				b := lift.Some(3).Try()
				return a * b
			})
			t.Run("Then Do returns Ok of the value", func(t *testing.T) {
				resEq(t, r.Must(), 6)
			})
		})
		t.Run("When a Result.Try fails", func(t *testing.T) {
			after := false
			r := lift.Do(func() int {
				_ = lift.Err[int](resErrBoom).Try()
				after = true
				return 1
			})
			t.Run("Then Do returns the error and the code after Try does not run", func(t *testing.T) {
				resEq(t, r.Err(), resErrBoom)
				resEq(t, after, false)
			})
		})
		t.Run("When an Option.Try gets None", func(t *testing.T) {
			after := false
			r := lift.Do(func() int {
				_ = lift.None[int]().Try()
				after = true
				return 1
			})
			t.Run("Then Do returns ErrNone and the code after Try does not run", func(t *testing.T) {
				resEq(t, errors.Is(r.Err(), lift.ErrNone), true)
				resEq(t, after, false)
			})
		})
		t.Run("When f writes to a nil map", func(t *testing.T) {
			f := func() {
				lift.Do(func() int {
					var m map[string]int
					m["a"] = 1
					return 0
				})
			}
			t.Run("Then Do does not recover the panic", func(t *testing.T) {
				resMustPanic(t, f)
			})
		})
		t.Run("When f calls panic(\"x\")", func(t *testing.T) {
			t.Run("Then Do passes the same panic value through", func(t *testing.T) {
				resPanicWith(t, func() { lift.Do(func() int { panic("x") }) }, "x")
			})
		})
		t.Run("When Do blocks are nested", func(t *testing.T) {
			innerAfter := false
			outer := lift.Do(func() int {
				inner := lift.Do(func() int {
					_ = lift.Err[int](resErrBoom).Try()
					innerAfter = true
					return 0
				})
				resEq(t, inner.Err(), resErrBoom)
				return inner.ValueOr(10) + 1
			})
			t.Run("Then the inner failure stops only the inner block", func(t *testing.T) {
				resEq(t, innerAfter, false)
				resEq(t, outer.Must(), 11)
			})
		})
		t.Run("When an inner Do succeeds and the outer Try fails", func(t *testing.T) {
			other := errors.New("other")
			outer := lift.Do(func() int {
				inner := lift.Do(func() int { return 5 })
				_ = lift.Err[int](other).Try()
				return inner.Must()
			})
			t.Run("Then the outer Do returns the outer error", func(t *testing.T) {
				resEq(t, outer.Err(), other)
			})
		})
	})
}

func TestDoRecover(t *testing.T) {
	t.Run("Given a DoRecover block", func(t *testing.T) {
		t.Run("When every Try succeeds", func(t *testing.T) {
			r := lift.DoRecover(func() string { return lift.Ok("a").Try() + lift.Some("b").Try() })
			t.Run("Then DoRecover returns Ok of the value", func(t *testing.T) {
				resEq(t, r.Must(), "ab")
			})
		})
		t.Run("When a Result.Try fails", func(t *testing.T) {
			r := lift.DoRecover(func() int { return lift.Err[int](resErrBoom).Try() })
			t.Run("Then DoRecover returns the error", func(t *testing.T) {
				resEq(t, r.Err(), resErrBoom)
			})
		})
		t.Run("When an Option.Try gets None", func(t *testing.T) {
			r := lift.DoRecover(func() int { return lift.None[int]().Try() })
			t.Run("Then DoRecover returns ErrNone", func(t *testing.T) {
				resEq(t, errors.Is(r.Err(), lift.ErrNone), true)
			})
		})
		t.Run("When f writes to a nil map", func(t *testing.T) {
			r := lift.DoRecover(func() int {
				var m map[string]int
				m["a"] = 1
				return 0
			})
			t.Run("Then DoRecover returns a *PanicError that unwraps to a runtime.Error", func(t *testing.T) {
				var pe *lift.PanicError
				resEq(t, errors.As(r.Err(), &pe), true)
				resNotEmpty(t, pe.Stack)
				var re runtime.Error
				resEq(t, errors.As(r.Err(), &re), true)
				resEq(t, pe.Unwrap(), error(re))
				resPrefix(t, pe.Error(), "panic: assignment to entry in nil map")
				resNotContains(t, pe.Error(), "original goroutine stack")
			})
		})
		t.Run("When f reads a slice out of range", func(t *testing.T) {
			r := lift.DoRecover(func() int {
				s := []int{}
				i := 3
				return s[i]
			})
			t.Run("Then errors.As finds the runtime.Error", func(t *testing.T) {
				var re runtime.Error
				resEq(t, errors.As(r.Err(), &re), true)
			})
		})
		t.Run("When f calls panic(\"x\")", func(t *testing.T) {
			r := lift.DoRecover(func() int { panic("x") })
			t.Run("Then the *PanicError holds \"x\", a stack, and Unwrap gives nil", func(t *testing.T) {
				var pe *lift.PanicError
				resEq(t, errors.As(r.Err(), &pe), true)
				resEq(t, pe.Value, "x")
				resNotEmpty(t, pe.Stack)
				resNil(t, pe.Unwrap())
				resEq(t, pe.Error(), "panic: x")
			})
		})
		t.Run("When f panics with an error value", func(t *testing.T) {
			r := lift.DoRecover(func() int { panic(resErrBoom) })
			t.Run("Then errors.Is finds the error through the *PanicError", func(t *testing.T) {
				resEq(t, errors.Is(r.Err(), resErrBoom), true)
			})
		})
	})
}

func TestDoOption(t *testing.T) {
	t.Run("Given a DoOption block", func(t *testing.T) {
		t.Run("When every Option.Try succeeds", func(t *testing.T) {
			o := lift.DoOption(func() int { return lift.Some(2).Try() + 1 })
			t.Run("Then DoOption returns Some of the value", func(t *testing.T) {
				resEq(t, o.Must(), 3)
			})
		})
		t.Run("When an Option.Try gets None", func(t *testing.T) {
			after := false
			o := lift.DoOption(func() int {
				_ = lift.None[int]().Try()
				after = true
				return 1
			})
			t.Run("Then DoOption returns None and the code after Try does not run", func(t *testing.T) {
				resEq(t, o.IsNone(), true)
				resEq(t, after, false)
			})
		})
		t.Run("When a Result.Try fails inside DoOption", func(t *testing.T) {
			f := func() { lift.DoOption(func() int { return lift.Err[int](resErrBoom).Try() }) }
			t.Run("Then DoOption panics", func(t *testing.T) {
				resMustPanic(t, f)
			})
		})
		t.Run("When f calls panic(\"x\")", func(t *testing.T) {
			t.Run("Then DoOption passes the panic through", func(t *testing.T) {
				resPanicWith(t, func() { lift.DoOption(func() int { panic("x") }) }, "x")
			})
		})
	})
}

func TestTryOutsideDo(t *testing.T) {
	t.Run("Given a Try call outside any Do", func(t *testing.T) {
		t.Run("When Result.Try gets a failure", func(t *testing.T) {
			p := resRecover(func() { lift.Err[int](resErrBoom).Try() })
			t.Run("Then it panics with a message that names lift.Do", func(t *testing.T) {
				err, ok := p.(error)
				resEq(t, ok, true)
				resContains(t, err.Error(), "outside lift.Do")
				resContains(t, err.Error(), "boom")
			})
		})
		t.Run("When Option.Try gets None", func(t *testing.T) {
			p := resRecover(func() { lift.None[int]().Try() })
			t.Run("Then it panics with a message that names lift.Do", func(t *testing.T) {
				err, ok := p.(error)
				resEq(t, ok, true)
				resContains(t, err.Error(), "outside lift.Do")
			})
		})
		t.Run("When Try gets a success", func(t *testing.T) {
			t.Run("Then it returns the value without a panic", func(t *testing.T) {
				resEq(t, lift.Ok(1).Try(), 1)
				resEq(t, strings.Repeat(lift.Some("a").Try(), 2), "aa")
			})
		})
	})
}
