package monadz_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/zkrebbekx/monadz"
)

var resErrBoom = errors.New("boom")

func TestResultConstructors(t *testing.T) {
	t.Run("Given the Result constructors", func(t *testing.T) {
		t.Run("When Ok is called with 7", func(t *testing.T) {
			r := monadz.Ok(7)
			t.Run("Then the Result is a success that holds 7", func(t *testing.T) {
				wantEq(t, r.IsOk(), true)
				wantEq(t, r.IsErr(), false)
				v, err := r.Get()
				wantEq(t, v, 7)
				wantNil(t, err)
				wantNil(t, r.Err())
			})
		})
		t.Run("When Err is called with a non-nil error", func(t *testing.T) {
			r := monadz.Err[int](resErrBoom)
			t.Run("Then the Result is a failure that holds the error", func(t *testing.T) {
				wantEq(t, r.IsErr(), true)
				wantEq(t, r.Err(), resErrBoom)
			})
		})
		t.Run("When Err is called with a nil error", func(t *testing.T) {
			r := monadz.Err[int](nil)
			t.Run("Then the Result is a failure that holds ErrNilError", func(t *testing.T) {
				wantEq(t, r.IsOk(), false)
				wantEq(t, errors.Is(r.Err(), monadz.ErrNilError), true)
			})
		})
		t.Run("When Errf is called with a format", func(t *testing.T) {
			r := monadz.Errf[int]("bad %d: %w", 3, resErrBoom)
			t.Run("Then the error has the formatted message and wraps the cause", func(t *testing.T) {
				wantEq(t, r.Err().Error(), "bad 3: boom")
				wantEq(t, errors.Is(r.Err(), resErrBoom), true)
			})
		})
		t.Run("When Of is called with a nil error", func(t *testing.T) {
			r := monadz.Of(strconv.Atoi("42"))
			t.Run("Then the Result is Ok(42)", func(t *testing.T) {
				wantEq(t, r.Must(), 42)
			})
		})
		t.Run("When Of is called with a non-nil error", func(t *testing.T) {
			r := monadz.Of(strconv.Atoi("x"))
			t.Run("Then the Result is a failure with the *strconv.NumError", func(t *testing.T) {
				var ne *strconv.NumError
				wantEq(t, errors.As(r.Err(), &ne), true)
			})
		})
		t.Run("When FromErr is called with nil and with an error", func(t *testing.T) {
			ok := monadz.FromErr(nil)
			bad := monadz.FromErr(resErrBoom)
			t.Run("Then nil gives Ok(Unit) and the error gives a failure", func(t *testing.T) {
				wantEq(t, ok.IsOk(), true)
				wantDeep(t, ok.Must(), monadz.Unit{})
				wantEq(t, bad.Err(), resErrBoom)
			})
		})
		t.Run("When the zero value of Result is used", func(t *testing.T) {
			var r monadz.Result[string]
			t.Run("Then it is Ok of the zero value", func(t *testing.T) {
				wantEq(t, r.IsOk(), true)
				wantEq(t, r.Must(), "")
			})
		})
	})
}

func TestResultQueries(t *testing.T) {
	t.Run("Given an Ok(4) and an Err(boom)", func(t *testing.T) {
		ok := monadz.Ok(4)
		bad := monadz.Err[int](resErrBoom)
		even := func(n int) bool { return n%2 == 0 }
		isBoom := func(err error) bool { return errors.Is(err, resErrBoom) }
		t.Run("When IsOkAnd and IsErrAnd are called", func(t *testing.T) {
			t.Run("Then they check the predicate only on the matching track", func(t *testing.T) {
				wantEq(t, ok.IsOkAnd(even), true)
				wantEq(t, ok.IsOkAnd(func(int) bool { return false }), false)
				wantEq(t, bad.IsOkAnd(func(int) bool { panic("not called") }), false)
				wantEq(t, bad.IsErrAnd(isBoom), true)
				wantEq(t, bad.IsErrAnd(func(error) bool { return false }), false)
				wantEq(t, ok.IsErrAnd(func(error) bool { panic("not called") }), false)
			})
		})
		t.Run("When Get is called on the failure", func(t *testing.T) {
			v, err := monadz.Err[string](resErrBoom).Get()
			t.Run("Then it returns the zero value and the error", func(t *testing.T) {
				wantEq(t, v, "")
				wantEq(t, err, resErrBoom)
			})
		})
		t.Run("When Ok() converts to an Option", func(t *testing.T) {
			t.Run("Then Ok gives Some and Err gives None", func(t *testing.T) {
				wantEq(t, ok.Ok().Must(), 4)
				wantEq(t, bad.Ok().IsNone(), true)
			})
		})
	})
}

func TestResultExtraction(t *testing.T) {
	t.Run("Given an Ok(5) and an Err(boom)", func(t *testing.T) {
		ok := monadz.Ok(5)
		bad := monadz.Err[int](resErrBoom)
		t.Run("When Must is called", func(t *testing.T) {
			t.Run("Then Ok returns 5 and Err panics with the error", func(t *testing.T) {
				wantEq(t, ok.Must(), 5)
				wantPanic(t, func() { bad.Must() }, resErrBoom)
			})
		})
		t.Run("When Expect is called", func(t *testing.T) {
			t.Run("Then Ok returns 5 and Err panics with the message and the cause", func(t *testing.T) {
				wantEq(t, ok.Expect("load"), 5)
				p := recovered(func() { bad.Expect("load config") })
				err, isErr := p.(error)
				wantEq(t, isErr, true)
				wantEq(t, err.Error(), "load config: boom")
				wantEq(t, errors.Is(err, resErrBoom), true)
			})
		})
		t.Run("When MustErr is called", func(t *testing.T) {
			t.Run("Then Err returns the error and Ok panics", func(t *testing.T) {
				wantEq(t, bad.MustErr(), resErrBoom)
				wantPanic(t, func() { ok.MustErr() }, "monadz: MustErr on Ok(5)")
			})
		})
		t.Run("When ValueOr is called", func(t *testing.T) {
			t.Run("Then Ok gives the value and Err gives the fallback", func(t *testing.T) {
				wantEq(t, ok.ValueOr(9), 5)
				wantEq(t, bad.ValueOr(9), 9)
			})
		})
		t.Run("When ValueOrElse is called", func(t *testing.T) {
			calls := 0
			f := func(err error) int { calls++; return len(err.Error()) }
			t.Run("Then the function runs only for Err and receives the error", func(t *testing.T) {
				wantEq(t, ok.ValueOrElse(f), 5)
				wantEq(t, calls, 0)
				wantEq(t, bad.ValueOrElse(f), 4)
				wantEq(t, calls, 1)
			})
		})
		t.Run("When ValueOrZero is called", func(t *testing.T) {
			t.Run("Then Ok gives the value and Err gives 0", func(t *testing.T) {
				wantEq(t, ok.ValueOrZero(), 5)
				wantEq(t, bad.ValueOrZero(), 0)
			})
		})
	})
}

func TestResultTransformation(t *testing.T) {
	t.Run("Given a pipeline of steps", func(t *testing.T) {
		t.Run("When Then applies strconv.Atoi to Ok(\"12\")", func(t *testing.T) {
			r := monadz.Ok("12").Then(strconv.Atoi)
			t.Run("Then the Result is Ok(12)", func(t *testing.T) {
				wantEq(t, r.Must(), 12)
			})
		})
		t.Run("When Then applies strconv.Atoi to Ok(\"x\")", func(t *testing.T) {
			r := monadz.Ok("x").Then(strconv.Atoi)
			t.Run("Then the Result is a failure", func(t *testing.T) {
				wantEq(t, r.IsErr(), true)
			})
		})
		t.Run("When Map and AndThen run on Ok values", func(t *testing.T) {
			r := monadz.Ok(3).
				Map(func(n int) string { return strconv.Itoa(n * 2) }).
				AndThen(func(s string) monadz.Result[string] { return monadz.Ok(s + "!") })
			t.Run("Then each step changes the value", func(t *testing.T) {
				wantEq(t, r.Must(), "6!")
			})
		})
		t.Run("When the first step fails", func(t *testing.T) {
			calls := 0
			r := monadz.Ok("x").
				Then(strconv.Atoi).
				Map(func(n int) int { calls++; return n }).
				Then(func(n int) (int, error) { calls++; return n, nil }).
				AndThen(func(n int) monadz.Result[int] { calls++; return monadz.Ok(n) }).
				Inspect(func(int) { calls++ })
			t.Run("Then no later step function is called", func(t *testing.T) {
				wantEq(t, calls, 0)
				var ne *strconv.NumError
				wantEq(t, errors.As(r.Err(), &ne), true)
			})
		})
		t.Run("When AndThen returns a failure", func(t *testing.T) {
			r := monadz.Ok(1).AndThen(func(int) monadz.Result[int] { return monadz.Err[int](resErrBoom) })
			t.Run("Then the chain is a failure", func(t *testing.T) {
				wantEq(t, r.Err(), resErrBoom)
			})
		})
		t.Run("When MapErr, Wrap, and Wrapf change the error", func(t *testing.T) {
			bad := monadz.Err[int](resErrBoom)
			mapped := bad.MapErr(func(e error) error { return fmt.Errorf("mapped: %w", e) })
			wrapped := bad.Wrap("load")
			wrappedf := bad.Wrapf("load %s", "cfg")
			t.Run("Then the messages change and errors.Is still finds the cause", func(t *testing.T) {
				wantEq(t, mapped.Err().Error(), "mapped: boom")
				wantEq(t, wrapped.Err().Error(), "load: boom")
				wantEq(t, wrappedf.Err().Error(), "load cfg: boom")
				wantEq(t, errors.Is(mapped.Err(), resErrBoom), true)
				wantEq(t, errors.Is(wrapped.Err(), resErrBoom), true)
				wantEq(t, errors.Is(wrappedf.Err(), resErrBoom), true)
			})
		})
		t.Run("When MapErr returns nil", func(t *testing.T) {
			r := monadz.Err[int](resErrBoom).MapErr(func(error) error { return nil })
			t.Run("Then the Result stays a failure with ErrNilError", func(t *testing.T) {
				wantEq(t, errors.Is(r.Err(), monadz.ErrNilError), true)
			})
		})
		t.Run("When MapErr, Wrap, and Wrapf run on Ok", func(t *testing.T) {
			ok := monadz.Ok(1)
			t.Run("Then the Result does not change and MapErr's function is not called", func(t *testing.T) {
				wantEq(t, ok.MapErr(func(error) error { panic("not called") }).Must(), 1)
				wantEq(t, ok.Wrap("x").Must(), 1)
				wantEq(t, ok.Wrapf("x %d", 1).Must(), 1)
			})
		})
		t.Run("When Ensure checks a predicate", func(t *testing.T) {
			positive := func(n int) bool { return n > 0 }
			t.Run("Then a passing value stays Ok", func(t *testing.T) {
				wantEq(t, monadz.Ok(1).Ensure(positive, resErrBoom).Must(), 1)
			})
			t.Run("Then a failing value becomes Err(err)", func(t *testing.T) {
				wantEq(t, monadz.Ok(-1).Ensure(positive, resErrBoom).Err(), resErrBoom)
			})
			t.Run("Then a failing value with a nil err becomes ErrNilError", func(t *testing.T) {
				wantEq(t, errors.Is(monadz.Ok(-1).Ensure(positive, nil).Err(), monadz.ErrNilError), true)
			})
			t.Run("Then a failure keeps its error and the predicate is not called", func(t *testing.T) {
				r := monadz.Err[int](resErrBoom).Ensure(func(int) bool { panic("not called") }, errors.New("other"))
				wantEq(t, r.Err(), resErrBoom)
			})
		})
		t.Run("When MapOr, MapOrElse, and Match run", func(t *testing.T) {
			ok := monadz.Ok(2)
			bad := monadz.Err[int](resErrBoom)
			double := func(n int) string { return strconv.Itoa(n * 2) }
			onErr := func(e error) string { return "err:" + e.Error() }
			t.Run("Then each picks the function of its track", func(t *testing.T) {
				wantEq(t, ok.MapOr("none", double), "4")
				wantEq(t, bad.MapOr("none", double), "none")
				wantEq(t, ok.MapOrElse(onErr, double), "4")
				wantEq(t, bad.MapOrElse(onErr, double), "err:boom")
				wantEq(t, ok.Match(double, onErr), "4")
				wantEq(t, bad.Match(double, onErr), "err:boom")
			})
		})
		t.Run("When Inspect and InspectErr run", func(t *testing.T) {
			var seen []string
			ok := monadz.Ok(1).Inspect(func(n int) { seen = append(seen, "ok") }).InspectErr(func(error) { seen = append(seen, "okerr") })
			bad := monadz.Err[int](resErrBoom).Inspect(func(int) { seen = append(seen, "bad") }).InspectErr(func(e error) { seen = append(seen, e.Error()) })
			t.Run("Then only the function of the matching track runs, and the Result does not change", func(t *testing.T) {
				wantDeep(t, seen, []string{"ok", "boom"})
				wantEq(t, ok.Must(), 1)
				wantEq(t, bad.Err(), resErrBoom)
			})
		})
	})
}

func TestResultRecovery(t *testing.T) {
	t.Run("Given an Ok(1) and an Err(boom)", func(t *testing.T) {
		ok := monadz.Ok(1)
		bad := monadz.Err[int](resErrBoom)
		other := errors.New("other")
		t.Run("When Or is called", func(t *testing.T) {
			t.Run("Then Ok keeps itself and Err gives the other Result", func(t *testing.T) {
				wantEq(t, ok.Or(monadz.Ok(2)).Must(), 1)
				wantEq(t, bad.Or(monadz.Ok(2)).Must(), 2)
				wantEq(t, bad.Or(monadz.Err[int](other)).Err(), other)
			})
		})
		t.Run("When OrElse is called", func(t *testing.T) {
			t.Run("Then the function runs only for Err and receives the error", func(t *testing.T) {
				wantEq(t, ok.OrElse(func(error) monadz.Result[int] { panic("not called") }).Must(), 1)
				r := bad.OrElse(func(e error) monadz.Result[int] { return monadz.Ok(len(e.Error())) })
				wantEq(t, r.Must(), 4)
			})
		})
		t.Run("When Recover is called", func(t *testing.T) {
			t.Run("Then a returned value recovers and a returned error stays a failure", func(t *testing.T) {
				wantEq(t, ok.Recover(func(error) (int, error) { panic("not called") }).Must(), 1)
				wantEq(t, bad.Recover(func(error) (int, error) { return 9, nil }).Must(), 9)
				r := bad.Recover(func(e error) (int, error) { return 0, fmt.Errorf("again: %w", e) })
				wantEq(t, r.Err().Error(), "again: boom")
				wantEq(t, errors.Is(r.Err(), resErrBoom), true)
			})
		})
	})
}

func TestResultCombination(t *testing.T) {
	t.Run("Given Ok and Err Results of different types", func(t *testing.T) {
		other := errors.New("other")
		t.Run("When And is called", func(t *testing.T) {
			t.Run("Then Ok gives the other Result and Err keeps its failure", func(t *testing.T) {
				wantEq(t, monadz.Ok(1).And(monadz.Ok("a")).Must(), "a")
				wantEq(t, monadz.Ok(1).And(monadz.Err[string](other)).Err(), other)
				wantEq(t, monadz.Err[int](resErrBoom).And(monadz.Ok("a")).Err(), resErrBoom)
			})
		})
		t.Run("When ZipWith is called", func(t *testing.T) {
			join := func(n int, s string) string { return strconv.Itoa(n) + s }
			t.Run("Then two Oks combine and the first failure wins", func(t *testing.T) {
				wantEq(t, monadz.Ok(1).ZipWith(monadz.Ok("a"), join).Must(), "1a")
				wantEq(t, monadz.Err[int](resErrBoom).ZipWith(monadz.Err[string](other), join).Err(), resErrBoom)
				wantEq(t, monadz.Ok(1).ZipWith(monadz.Err[string](other), join).Err(), other)
			})
		})
	})
}

func TestResultConversion(t *testing.T) {
	t.Run("Given an Ok(3) and an Err(boom)", func(t *testing.T) {
		ok := monadz.Ok(3)
		bad := monadz.Err[int](resErrBoom)
		t.Run("When Either is called", func(t *testing.T) {
			t.Run("Then Ok gives Right and Err gives Left", func(t *testing.T) {
				wantEq(t, ok.Either().IsRight(), true)
				wantEq(t, ok.Either().Right().Must(), 3)
				wantEq(t, bad.Either().IsLeft(), true)
				wantEq(t, bad.Either().Left().Must(), resErrBoom)
			})
		})
		t.Run("When Validation is called", func(t *testing.T) {
			t.Run("Then Ok gives Valid and Err gives Invalid with the error", func(t *testing.T) {
				v, err := ok.Validation().Get()
				wantEq(t, v, 3)
				wantNil(t, err)
				wantDeep(t, bad.Validation().Errors(), []error{resErrBoom})
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
				wantDeep(t, got, []int{3})
			})
		})
		t.Run("When All is ranged over with an early break", func(t *testing.T) {
			n := 0
			for range ok.All() {
				n++
				break
			}
			t.Run("Then the loop stops without a panic", func(t *testing.T) {
				wantEq(t, n, 1)
			})
		})
	})
}

func TestResultFormatting(t *testing.T) {
	t.Run("Given an Ok(3) and an Err(boom)", func(t *testing.T) {
		ok := monadz.Ok(3)
		bad := monadz.Err[int](resErrBoom)
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then it gives Ok(3) and Err(boom)", func(t *testing.T) {
				wantEq(t, ok.String(), "Ok(3)")
				wantEq(t, bad.String(), "Err(boom)")
				wantEq(t, fmt.Sprint(bad), "Err(boom)")
			})
		})
		t.Run("When the Results are logged through a slog JSON handler", func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			logger.Info("msg", "ok", ok, "bad", bad)
			var rec map[string]any
			err := json.Unmarshal(buf.Bytes(), &rec)
			t.Run("Then Ok logs the value and Err logs a group with the error message", func(t *testing.T) {
				wantNil(t, err)
				wantEq(t, rec["ok"], any(float64(3)))
				wantDeep(t, rec["bad"], map[string]any{"error": "boom"})
				wantEq(t, strings.Contains(buf.String(), `"bad":{"error":"boom"}`), true)
			})
		})
	})
}
