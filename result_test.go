package lift_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/zkrebbekx/lift"
)

var resErrBoom = errors.New("boom")

// resRecover runs f and returns the value that f panics with.
func resRecover(f func()) (p any) {
	defer func() { p = recover() }()
	f()
	return nil
}

func TestResultConstructors(t *testing.T) {
	t.Run("Given the Result constructors", func(t *testing.T) {
		t.Run("When Ok is called with 7", func(t *testing.T) {
			r := lift.Ok(7)
			t.Run("Then the Result is a success that holds 7", func(t *testing.T) {
				resEq(t, r.IsOk(), true)
				resEq(t, r.IsErr(), false)
				v, err := r.Get()
				resEq(t, v, 7)
				resNil(t, err)
				resNil(t, r.Err())
			})
		})
		t.Run("When Err is called with a non-nil error", func(t *testing.T) {
			r := lift.Err[int](resErrBoom)
			t.Run("Then the Result is a failure that holds the error", func(t *testing.T) {
				resEq(t, r.IsErr(), true)
				resEq(t, r.Err(), resErrBoom)
			})
		})
		t.Run("When Err is called with a nil error", func(t *testing.T) {
			r := lift.Err[int](nil)
			t.Run("Then the Result is a failure that holds ErrNilError", func(t *testing.T) {
				resEq(t, r.IsOk(), false)
				resEq(t, errors.Is(r.Err(), lift.ErrNilError), true)
			})
		})
		t.Run("When Errf is called with a format", func(t *testing.T) {
			r := lift.Errf[int]("bad %d: %w", 3, resErrBoom)
			t.Run("Then the error has the formatted message and wraps the cause", func(t *testing.T) {
				resEq(t, r.Err().Error(), "bad 3: boom")
				resEq(t, errors.Is(r.Err(), resErrBoom), true)
			})
		})
		t.Run("When Of is called with a nil error", func(t *testing.T) {
			r := lift.Of(strconv.Atoi("42"))
			t.Run("Then the Result is Ok(42)", func(t *testing.T) {
				resEq(t, r.Must(), 42)
			})
		})
		t.Run("When Of is called with a non-nil error", func(t *testing.T) {
			r := lift.Of(strconv.Atoi("x"))
			t.Run("Then the Result is a failure with the *strconv.NumError", func(t *testing.T) {
				var ne *strconv.NumError
				resEq(t, errors.As(r.Err(), &ne), true)
			})
		})
		t.Run("When FromErr is called with nil and with an error", func(t *testing.T) {
			ok := lift.FromErr(nil)
			bad := lift.FromErr(resErrBoom)
			t.Run("Then nil gives Ok(Unit) and the error gives a failure", func(t *testing.T) {
				resEq(t, ok.IsOk(), true)
				resDeep(t, ok.Must(), lift.Unit{})
				resEq(t, bad.Err(), resErrBoom)
			})
		})
		t.Run("When the zero value of Result is used", func(t *testing.T) {
			var r lift.Result[string]
			t.Run("Then it is Ok of the zero value", func(t *testing.T) {
				resEq(t, r.IsOk(), true)
				resEq(t, r.Must(), "")
			})
		})
	})
}

func TestResultQueries(t *testing.T) {
	t.Run("Given an Ok(4) and an Err(boom)", func(t *testing.T) {
		ok := lift.Ok(4)
		bad := lift.Err[int](resErrBoom)
		even := func(n int) bool { return n%2 == 0 }
		isBoom := func(err error) bool { return errors.Is(err, resErrBoom) }
		t.Run("When IsOkAnd and IsErrAnd are called", func(t *testing.T) {
			t.Run("Then they check the predicate only on the matching track", func(t *testing.T) {
				resEq(t, ok.IsOkAnd(even), true)
				resEq(t, ok.IsOkAnd(func(int) bool { return false }), false)
				resEq(t, bad.IsOkAnd(func(int) bool { panic("not called") }), false)
				resEq(t, bad.IsErrAnd(isBoom), true)
				resEq(t, bad.IsErrAnd(func(error) bool { return false }), false)
				resEq(t, ok.IsErrAnd(func(error) bool { panic("not called") }), false)
			})
		})
		t.Run("When Get is called on the failure", func(t *testing.T) {
			v, err := lift.Err[string](resErrBoom).Get()
			t.Run("Then it returns the zero value and the error", func(t *testing.T) {
				resEq(t, v, "")
				resEq(t, err, resErrBoom)
			})
		})
		t.Run("When Ok() converts to an Option", func(t *testing.T) {
			t.Run("Then Ok gives Some and Err gives None", func(t *testing.T) {
				resEq(t, ok.Ok().Must(), 4)
				resEq(t, bad.Ok().IsNone(), true)
			})
		})
	})
}

func TestResultExtraction(t *testing.T) {
	t.Run("Given an Ok(5) and an Err(boom)", func(t *testing.T) {
		ok := lift.Ok(5)
		bad := lift.Err[int](resErrBoom)
		t.Run("When Must is called", func(t *testing.T) {
			t.Run("Then Ok returns 5 and Err panics with the error", func(t *testing.T) {
				resEq(t, ok.Must(), 5)
				resPanicWith(t, func() { bad.Must() }, resErrBoom)
			})
		})
		t.Run("When Expect is called", func(t *testing.T) {
			t.Run("Then Ok returns 5 and Err panics with the message and the cause", func(t *testing.T) {
				resEq(t, ok.Expect("load"), 5)
				p := resRecover(func() { bad.Expect("load config") })
				err, isErr := p.(error)
				resEq(t, isErr, true)
				resEq(t, err.Error(), "load config: boom")
				resEq(t, errors.Is(err, resErrBoom), true)
			})
		})
		t.Run("When MustErr is called", func(t *testing.T) {
			t.Run("Then Err returns the error and Ok panics", func(t *testing.T) {
				resEq(t, bad.MustErr(), resErrBoom)
				resPanicWith(t, func() { ok.MustErr() }, "lift: MustErr on Ok(5)")
			})
		})
		t.Run("When ValueOr is called", func(t *testing.T) {
			t.Run("Then Ok gives the value and Err gives the fallback", func(t *testing.T) {
				resEq(t, ok.ValueOr(9), 5)
				resEq(t, bad.ValueOr(9), 9)
			})
		})
		t.Run("When ValueOrElse is called", func(t *testing.T) {
			calls := 0
			f := func(err error) int { calls++; return len(err.Error()) }
			t.Run("Then the function runs only for Err and receives the error", func(t *testing.T) {
				resEq(t, ok.ValueOrElse(f), 5)
				resEq(t, calls, 0)
				resEq(t, bad.ValueOrElse(f), 4)
				resEq(t, calls, 1)
			})
		})
		t.Run("When ValueOrZero is called", func(t *testing.T) {
			t.Run("Then Ok gives the value and Err gives 0", func(t *testing.T) {
				resEq(t, ok.ValueOrZero(), 5)
				resEq(t, bad.ValueOrZero(), 0)
			})
		})
	})
}

func TestResultTransformation(t *testing.T) {
	t.Run("Given a pipeline of steps", func(t *testing.T) {
		t.Run("When Then applies strconv.Atoi to Ok(\"12\")", func(t *testing.T) {
			r := lift.Ok("12").Then(strconv.Atoi)
			t.Run("Then the Result is Ok(12)", func(t *testing.T) {
				resEq(t, r.Must(), 12)
			})
		})
		t.Run("When Then applies strconv.Atoi to Ok(\"x\")", func(t *testing.T) {
			r := lift.Ok("x").Then(strconv.Atoi)
			t.Run("Then the Result is a failure", func(t *testing.T) {
				resEq(t, r.IsErr(), true)
			})
		})
		t.Run("When Map and AndThen run on Ok values", func(t *testing.T) {
			r := lift.Ok(3).
				Map(func(n int) string { return strconv.Itoa(n * 2) }).
				AndThen(func(s string) lift.Result[string] { return lift.Ok(s + "!") })
			t.Run("Then each step changes the value", func(t *testing.T) {
				resEq(t, r.Must(), "6!")
			})
		})
		t.Run("When the first step fails", func(t *testing.T) {
			calls := 0
			r := lift.Ok("x").
				Then(strconv.Atoi).
				Map(func(n int) int { calls++; return n }).
				Then(func(n int) (int, error) { calls++; return n, nil }).
				AndThen(func(n int) lift.Result[int] { calls++; return lift.Ok(n) }).
				Inspect(func(int) { calls++ })
			t.Run("Then no later step function is called", func(t *testing.T) {
				resEq(t, calls, 0)
				var ne *strconv.NumError
				resEq(t, errors.As(r.Err(), &ne), true)
			})
		})
		t.Run("When AndThen returns a failure", func(t *testing.T) {
			r := lift.Ok(1).AndThen(func(int) lift.Result[int] { return lift.Err[int](resErrBoom) })
			t.Run("Then the chain is a failure", func(t *testing.T) {
				resEq(t, r.Err(), resErrBoom)
			})
		})
		t.Run("When MapErr, Wrap, and Wrapf change the error", func(t *testing.T) {
			bad := lift.Err[int](resErrBoom)
			mapped := bad.MapErr(func(e error) error { return fmt.Errorf("mapped: %w", e) })
			wrapped := bad.Wrap("load")
			wrappedf := bad.Wrapf("load %s", "cfg")
			t.Run("Then the messages change and errors.Is still finds the cause", func(t *testing.T) {
				resEq(t, mapped.Err().Error(), "mapped: boom")
				resEq(t, wrapped.Err().Error(), "load: boom")
				resEq(t, wrappedf.Err().Error(), "load cfg: boom")
				resEq(t, errors.Is(mapped.Err(), resErrBoom), true)
				resEq(t, errors.Is(wrapped.Err(), resErrBoom), true)
				resEq(t, errors.Is(wrappedf.Err(), resErrBoom), true)
			})
		})
		t.Run("When MapErr returns nil", func(t *testing.T) {
			r := lift.Err[int](resErrBoom).MapErr(func(error) error { return nil })
			t.Run("Then the Result stays a failure with ErrNilError", func(t *testing.T) {
				resEq(t, errors.Is(r.Err(), lift.ErrNilError), true)
			})
		})
		t.Run("When MapErr, Wrap, and Wrapf run on Ok", func(t *testing.T) {
			ok := lift.Ok(1)
			t.Run("Then the Result does not change and MapErr's function is not called", func(t *testing.T) {
				resEq(t, ok.MapErr(func(error) error { panic("not called") }).Must(), 1)
				resEq(t, ok.Wrap("x").Must(), 1)
				resEq(t, ok.Wrapf("x %d", 1).Must(), 1)
			})
		})
		t.Run("When Ensure checks a predicate", func(t *testing.T) {
			positive := func(n int) bool { return n > 0 }
			t.Run("Then a passing value stays Ok", func(t *testing.T) {
				resEq(t, lift.Ok(1).Ensure(positive, resErrBoom).Must(), 1)
			})
			t.Run("Then a failing value becomes Err(err)", func(t *testing.T) {
				resEq(t, lift.Ok(-1).Ensure(positive, resErrBoom).Err(), resErrBoom)
			})
			t.Run("Then a failing value with a nil err becomes ErrNilError", func(t *testing.T) {
				resEq(t, errors.Is(lift.Ok(-1).Ensure(positive, nil).Err(), lift.ErrNilError), true)
			})
			t.Run("Then a failure keeps its error and the predicate is not called", func(t *testing.T) {
				r := lift.Err[int](resErrBoom).Ensure(func(int) bool { panic("not called") }, errors.New("other"))
				resEq(t, r.Err(), resErrBoom)
			})
		})
		t.Run("When MapOr, MapOrElse, and Match run", func(t *testing.T) {
			ok := lift.Ok(2)
			bad := lift.Err[int](resErrBoom)
			double := func(n int) string { return strconv.Itoa(n * 2) }
			onErr := func(e error) string { return "err:" + e.Error() }
			t.Run("Then each picks the function of its track", func(t *testing.T) {
				resEq(t, ok.MapOr("none", double), "4")
				resEq(t, bad.MapOr("none", double), "none")
				resEq(t, ok.MapOrElse(onErr, double), "4")
				resEq(t, bad.MapOrElse(onErr, double), "err:boom")
				resEq(t, ok.Match(double, onErr), "4")
				resEq(t, bad.Match(double, onErr), "err:boom")
			})
		})
		t.Run("When Inspect and InspectErr run", func(t *testing.T) {
			var seen []string
			ok := lift.Ok(1).Inspect(func(n int) { seen = append(seen, "ok") }).InspectErr(func(error) { seen = append(seen, "okerr") })
			bad := lift.Err[int](resErrBoom).Inspect(func(int) { seen = append(seen, "bad") }).InspectErr(func(e error) { seen = append(seen, e.Error()) })
			t.Run("Then only the function of the matching track runs, and the Result does not change", func(t *testing.T) {
				resDeep(t, seen, []string{"ok", "boom"})
				resEq(t, ok.Must(), 1)
				resEq(t, bad.Err(), resErrBoom)
			})
		})
	})
}

func TestResultRecovery(t *testing.T) {
	t.Run("Given an Ok(1) and an Err(boom)", func(t *testing.T) {
		ok := lift.Ok(1)
		bad := lift.Err[int](resErrBoom)
		other := errors.New("other")
		t.Run("When Or is called", func(t *testing.T) {
			t.Run("Then Ok keeps itself and Err gives the other Result", func(t *testing.T) {
				resEq(t, ok.Or(lift.Ok(2)).Must(), 1)
				resEq(t, bad.Or(lift.Ok(2)).Must(), 2)
				resEq(t, bad.Or(lift.Err[int](other)).Err(), other)
			})
		})
		t.Run("When OrElse is called", func(t *testing.T) {
			t.Run("Then the function runs only for Err and receives the error", func(t *testing.T) {
				resEq(t, ok.OrElse(func(error) lift.Result[int] { panic("not called") }).Must(), 1)
				r := bad.OrElse(func(e error) lift.Result[int] { return lift.Ok(len(e.Error())) })
				resEq(t, r.Must(), 4)
			})
		})
		t.Run("When Recover is called", func(t *testing.T) {
			t.Run("Then a returned value recovers and a returned error stays a failure", func(t *testing.T) {
				resEq(t, ok.Recover(func(error) (int, error) { panic("not called") }).Must(), 1)
				resEq(t, bad.Recover(func(error) (int, error) { return 9, nil }).Must(), 9)
				r := bad.Recover(func(e error) (int, error) { return 0, fmt.Errorf("again: %w", e) })
				resEq(t, r.Err().Error(), "again: boom")
				resEq(t, errors.Is(r.Err(), resErrBoom), true)
			})
		})
	})
}

func TestResultCombination(t *testing.T) {
	t.Run("Given Ok and Err Results of different types", func(t *testing.T) {
		other := errors.New("other")
		t.Run("When And is called", func(t *testing.T) {
			t.Run("Then Ok gives the other Result and Err keeps its failure", func(t *testing.T) {
				resEq(t, lift.Ok(1).And(lift.Ok("a")).Must(), "a")
				resEq(t, lift.Ok(1).And(lift.Err[string](other)).Err(), other)
				resEq(t, lift.Err[int](resErrBoom).And(lift.Ok("a")).Err(), resErrBoom)
			})
		})
		t.Run("When ZipWith is called", func(t *testing.T) {
			join := func(n int, s string) string { return strconv.Itoa(n) + s }
			t.Run("Then two Oks combine and the first failure wins", func(t *testing.T) {
				resEq(t, lift.Ok(1).ZipWith(lift.Ok("a"), join).Must(), "1a")
				resEq(t, lift.Err[int](resErrBoom).ZipWith(lift.Err[string](other), join).Err(), resErrBoom)
				resEq(t, lift.Ok(1).ZipWith(lift.Err[string](other), join).Err(), other)
			})
		})
	})
}

func TestResultConversion(t *testing.T) {
	t.Run("Given an Ok(3) and an Err(boom)", func(t *testing.T) {
		ok := lift.Ok(3)
		bad := lift.Err[int](resErrBoom)
		t.Run("When Either is called", func(t *testing.T) {
			t.Run("Then Ok gives Right and Err gives Left", func(t *testing.T) {
				resEq(t, ok.Either().IsRight(), true)
				resEq(t, ok.Either().Right().Must(), 3)
				resEq(t, bad.Either().IsLeft(), true)
				resEq(t, bad.Either().Left().Must(), resErrBoom)
			})
		})
		t.Run("When Validation is called", func(t *testing.T) {
			t.Run("Then Ok gives Valid and Err gives Invalid with the error", func(t *testing.T) {
				v, err := ok.Validation().Get()
				resEq(t, v, 3)
				resNil(t, err)
				resDeep(t, bad.Validation().Errors(), []error{resErrBoom})
			})
		})
		t.Run("When All is ranged over", func(t *testing.T) {
			var got []int
			for v := range ok.All() {
				got = append(got, v)
			}
			for v := range bad.All() {
				got = append(got, v)
			}
			t.Run("Then Ok yields one value and Err yields none", func(t *testing.T) {
				resDeep(t, got, []int{3})
			})
		})
		t.Run("When All is ranged over with an early break", func(t *testing.T) {
			n := 0
			for range ok.All() {
				n++
				break
			}
			t.Run("Then the loop stops without a panic", func(t *testing.T) {
				resEq(t, n, 1)
			})
		})
	})
}

func TestResultFormatting(t *testing.T) {
	t.Run("Given an Ok(3) and an Err(boom)", func(t *testing.T) {
		ok := lift.Ok(3)
		bad := lift.Err[int](resErrBoom)
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then it gives Ok(3) and Err(boom)", func(t *testing.T) {
				resEq(t, ok.String(), "Ok(3)")
				resEq(t, bad.String(), "Err(boom)")
				resEq(t, fmt.Sprint(bad), "Err(boom)")
			})
		})
		t.Run("When the Results are logged through a slog JSON handler", func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			logger.Info("msg", "ok", ok, "bad", bad)
			var rec map[string]any
			err := json.Unmarshal(buf.Bytes(), &rec)
			t.Run("Then Ok logs the value and Err logs a group with the error message", func(t *testing.T) {
				resNil(t, err)
				resEq(t, rec["ok"], any(float64(3)))
				resDeep(t, rec["bad"], map[string]any{"error": "boom"})
				resEq(t, strings.Contains(buf.String(), `"bad":{"error":"boom"}`), true)
			})
		})
	})
}

// resEq fails the test if got != want.
func resEq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// resDeep fails the test if got and want are not deeply equal.
func resDeep(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// resNil fails the test if got is not nil.
func resNil(t *testing.T, got any) {
	t.Helper()
	if got != nil {
		t.Errorf("got %#v, want nil", got)
	}
}

// resNotEmpty fails the test if b is empty.
func resNotEmpty(t *testing.T, b []byte) {
	t.Helper()
	if len(b) == 0 {
		t.Errorf("got an empty slice, want a non-empty slice")
	}
}

func resContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("got %q, want it to contain %q", s, sub)
	}
}

func resNotContains(t *testing.T, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Errorf("got %q, want it not to contain %q", s, sub)
	}
}

func resPrefix(t *testing.T, s, prefix string) {
	t.Helper()
	if !strings.HasPrefix(s, prefix) {
		t.Errorf("got %q, want the prefix %q", s, prefix)
	}
}

// resMustPanic fails the test if f does not panic, and returns the
// recovered value.
func resMustPanic(t *testing.T, f func()) (p any) {
	t.Helper()
	panicked := true
	func() {
		defer func() { p = recover() }()
		f()
		panicked = false
	}()
	if !panicked {
		t.Errorf("got no panic, want a panic")
	}
	return p
}

// resPanicWith fails the test if f does not panic with want.
func resPanicWith(t *testing.T, f func(), want any) {
	t.Helper()
	if got := resMustPanic(t, f); got != want {
		t.Errorf("got panic %#v, want panic %#v", got, want)
	}
}
