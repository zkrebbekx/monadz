package monadz_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/zkrebbekx/monadz"
)

// The monad laws.
//
// A type is a lawful monad when its unit ("wrap a plain value") and its bind
// (AndThen) obey three rules. In the rules, a is a plain value, m is a
// wrapped value, and f and g are steps that take a plain value and return a
// wrapped value.
//
//   - Left identity: unit(a).AndThen(f) equals f(a). Wrapping a value and
//     then binding it does nothing more than calling the step directly.
//   - Right identity: m.AndThen(unit) equals m. Binding to unit changes
//     nothing.
//   - Associativity: m.AndThen(f).AndThen(g) equals
//     m.AndThen(func(x) { return f(x).AndThen(g) }). The grouping of a chain
//     does not change the result, so a long pipeline can be split into
//     helper functions in any way.
//
// The unit for each type: Some for Option, Ok for Result, Right for Either,
// StateOf for State, ReaderOf for Reader, and NewWriter with no log for
// Writer. State and Reader hold functions, so the tests compare them by
// running them on several inputs. Writer is compared by Run.

var (
	eitErrOdd  = errors.New("odd")
	eitErrNeg  = errors.New("negative")
	eitLawInts = []int{-3, -1, 0, 1, 2, 4, 7, 10}
)

func TestLawsOption(t *testing.T) {
	half := func(n int) monadz.Option[int] {
		if n%2 != 0 {
			return monadz.None[int]()
		}
		return monadz.Some(n / 2)
	}
	dec := func(n int) monadz.Option[int] {
		if n <= 0 {
			return monadz.None[int]()
		}
		return monadz.Some(n - 1)
	}
	t.Run("Given Option with Some as unit and two steps that can return None", func(t *testing.T) {
		ms := []monadz.Option[int]{monadz.None[int]()}
		for _, n := range eitLawInts {
			ms = append(ms, monadz.Some(n))
		}
		for _, a := range eitLawInts {
			t.Run(fmt.Sprintf("When left identity is checked for a=%d", a), func(t *testing.T) {
				t.Run("Then Some(a).AndThen(f) equals f(a)", func(t *testing.T) {
					wantDeep(t, monadz.Some(a).AndThen(half), half(a))
				})
			})
		}
		for _, m := range ms {
			t.Run(fmt.Sprintf("When right identity and associativity are checked for m=%v", m), func(t *testing.T) {
				t.Run("Then m.AndThen(Some) equals m", func(t *testing.T) {
					wantDeep(t, m.AndThen(monadz.Some[int]), m)
				})
				t.Run("Then the grouping of AndThen does not change the result", func(t *testing.T) {
					lhs := m.AndThen(half).AndThen(dec)
					rhs := m.AndThen(func(x int) monadz.Option[int] { return half(x).AndThen(dec) })
					wantDeep(t, lhs, rhs)
				})
			})
		}
	})
}

func TestLawsResult(t *testing.T) {
	half := func(n int) monadz.Result[int] {
		if n%2 != 0 {
			return monadz.Err[int](eitErrOdd)
		}
		return monadz.Ok(n / 2)
	}
	dec := func(n int) monadz.Result[int] {
		if n <= 0 {
			return monadz.Err[int](eitErrNeg)
		}
		return monadz.Ok(n - 1)
	}
	t.Run("Given Result with Ok as unit and two steps that can fail", func(t *testing.T) {
		ms := []monadz.Result[int]{monadz.Err[int](eitErrLoad)}
		for _, n := range eitLawInts {
			ms = append(ms, monadz.Ok(n))
		}
		for _, a := range eitLawInts {
			t.Run(fmt.Sprintf("When left identity is checked for a=%d", a), func(t *testing.T) {
				t.Run("Then Ok(a).AndThen(f) equals f(a)", func(t *testing.T) {
					wantDeep(t, monadz.Ok(a).AndThen(half), half(a))
				})
			})
		}
		for _, m := range ms {
			t.Run(fmt.Sprintf("When right identity and associativity are checked for m=%v", m), func(t *testing.T) {
				t.Run("Then m.AndThen(Ok) equals m", func(t *testing.T) {
					wantDeep(t, m.AndThen(monadz.Ok[int]), m)
				})
				t.Run("Then the grouping of AndThen does not change the result", func(t *testing.T) {
					lhs := m.AndThen(half).AndThen(dec)
					rhs := m.AndThen(func(x int) monadz.Result[int] { return half(x).AndThen(dec) })
					wantDeep(t, lhs, rhs)
				})
			})
		}
	})
}

func TestLawsEither(t *testing.T) {
	half := func(n int) monadz.Either[string, int] {
		if n%2 != 0 {
			return monadz.Left[string, int]("odd")
		}
		return monadz.Right[string](n / 2)
	}
	dec := func(n int) monadz.Either[string, int] {
		if n <= 0 {
			return monadz.Left[string, int]("not positive")
		}
		return monadz.Right[string](n - 1)
	}
	unit := monadz.Right[string, int]
	t.Run("Given Either with Right as unit and two steps that can return Left", func(t *testing.T) {
		ms := []monadz.Either[string, int]{monadz.Left[string, int]("start"), {}}
		for _, n := range eitLawInts {
			ms = append(ms, unit(n))
		}
		for _, a := range eitLawInts {
			t.Run(fmt.Sprintf("When left identity is checked for a=%d", a), func(t *testing.T) {
				t.Run("Then Right(a).AndThen(f) equals f(a)", func(t *testing.T) {
					wantDeep(t, unit(a).AndThen(half), half(a))
				})
			})
		}
		for _, m := range ms {
			t.Run(fmt.Sprintf("When right identity and associativity are checked for m=%v", m), func(t *testing.T) {
				t.Run("Then m.AndThen(Right) equals m", func(t *testing.T) {
					wantDeep(t, m.AndThen(unit), m)
				})
				t.Run("Then the grouping of AndThen does not change the result", func(t *testing.T) {
					lhs := m.AndThen(half).AndThen(dec)
					rhs := m.AndThen(func(x int) monadz.Either[string, int] { return half(x).AndThen(dec) })
					wantDeep(t, lhs, rhs)
				})
			})
		}
	})
}

// eitSameState fails the test if a and b give a different value or state
// for any start state.
func eitSameState[A any](t *testing.T, a, b monadz.State[int, A]) {
	t.Helper()
	for _, s := range eitLawInts {
		av, as := a.Run(s)
		bv, bs := b.Run(s)
		wantDeep(t, []any{av, as}, []any{bv, bs})
	}
}

func TestLawsState(t *testing.T) {
	// f adds the state to the value and increments the state.
	f := func(n int) monadz.State[int, int] {
		return monadz.NewState(func(s int) (int, int) { return n + s, s + 1 })
	}
	// g multiplies the value by the state and doubles the state.
	g := func(n int) monadz.State[int, string] {
		return monadz.NewState(func(s int) (string, int) { return fmt.Sprint(n * s), s * 2 })
	}
	t.Run("Given State with StateOf as unit and two steps that read and change the state", func(t *testing.T) {
		ms := map[string]monadz.State[int, int]{
			"StateOf(3)":  monadz.StateOf[int](3),
			"GetState":    monadz.GetState[int](),
			"tick":        monadz.NewState(func(s int) (int, int) { return s * 10, s - 1 }),
			"ModifyState": monadz.ModifyState(eitDouble).Map(func(monadz.Unit) int { return 1 }),
		}
		for _, a := range eitLawInts {
			t.Run(fmt.Sprintf("When left identity is checked for a=%d", a), func(t *testing.T) {
				t.Run("Then StateOf(a).AndThen(f) runs the same as f(a) on every start state", func(t *testing.T) {
					eitSameState(t, monadz.StateOf[int](a).AndThen(f), f(a))
				})
			})
		}
		for name, m := range ms {
			t.Run("When right identity and associativity are checked for m="+name, func(t *testing.T) {
				t.Run("Then m.AndThen(StateOf) runs the same as m", func(t *testing.T) {
					eitSameState(t, m.AndThen(monadz.StateOf[int, int]), m)
				})
				t.Run("Then the grouping of AndThen does not change the run", func(t *testing.T) {
					lhs := m.AndThen(f).AndThen(g)
					rhs := m.AndThen(func(x int) monadz.State[int, string] { return f(x).AndThen(g) })
					eitSameState(t, lhs, rhs)
				})
			})
		}
	})
}

// eitSameReader fails the test if a and b give a different value for any
// environment.
func eitSameReader[A any](t *testing.T, a, b monadz.Reader[int, A]) {
	t.Helper()
	for _, env := range eitLawInts {
		wantDeep(t, a.Run(env), b.Run(env))
	}
}

func TestLawsReader(t *testing.T) {
	f := func(n int) monadz.Reader[int, int] {
		return monadz.NewReader(func(env int) int { return n*100 + env })
	}
	g := func(n int) monadz.Reader[int, string] {
		return monadz.NewReader(func(env int) string { return fmt.Sprintf("%d/%d", n, env) })
	}
	t.Run("Given Reader with ReaderOf as unit and two steps that read the environment", func(t *testing.T) {
		ms := map[string]monadz.Reader[int, int]{
			"ReaderOf(5)": monadz.ReaderOf[int](5),
			"Ask":         monadz.Ask[int](),
			"NewReader":   monadz.NewReader(func(env int) int { return env * env }),
			"Local":       monadz.Ask[int]().Local(func(env int) int { return env + 1 }),
		}
		for _, a := range eitLawInts {
			t.Run(fmt.Sprintf("When left identity is checked for a=%d", a), func(t *testing.T) {
				t.Run("Then ReaderOf(a).AndThen(f) runs the same as f(a)", func(t *testing.T) {
					eitSameReader(t, monadz.ReaderOf[int](a).AndThen(f), f(a))
				})
			})
		}
		for name, m := range ms {
			t.Run("When right identity and associativity are checked for m="+name, func(t *testing.T) {
				t.Run("Then m.AndThen(ReaderOf) runs the same as m", func(t *testing.T) {
					eitSameReader(t, m.AndThen(monadz.ReaderOf[int, int]), m)
				})
				t.Run("Then the grouping of AndThen does not change the run", func(t *testing.T) {
					lhs := m.AndThen(f).AndThen(g)
					rhs := m.AndThen(func(x int) monadz.Reader[int, string] { return f(x).AndThen(g) })
					eitSameReader(t, lhs, rhs)
				})
			})
		}
	})
}

// eitSameWriter fails the test if a and b give a different value or log.
// An empty log and a nil log count as the same.
func eitSameWriter[A any](t *testing.T, a, b monadz.Writer[string, A]) {
	t.Helper()
	av, al := a.Run()
	bv, bl := b.Run()
	if len(al) == 0 && len(bl) == 0 {
		al, bl = nil, nil
	}
	wantDeep(t, []any{av, al}, []any{bv, bl})
}

func TestLawsWriter(t *testing.T) {
	unit := func(n int) monadz.Writer[string, int] { return monadz.NewWriter[string](n) }
	f := func(n int) monadz.Writer[string, int] {
		return monadz.NewWriter(n+1, fmt.Sprintf("f(%d)", n))
	}
	g := func(n int) monadz.Writer[string, string] {
		if n%2 == 0 {
			return monadz.NewWriter[string](fmt.Sprint(n))
		}
		return monadz.NewWriter(fmt.Sprint(n), "g:odd", fmt.Sprintf("g(%d)", n))
	}
	t.Run("Given Writer with an empty-log NewWriter as unit and two logging steps", func(t *testing.T) {
		ms := map[string]monadz.Writer[string, int]{
			"unit(4)":      unit(4),
			"logged":       monadz.NewWriter(3, "a", "b"),
			"Tell+Map":     monadz.Tell("t").Map(func(monadz.Unit) int { return 0 }),
			"Censored":     monadz.NewWriter(9, "x").Censor(func([]string) []string { return nil }),
			"chained(7,8)": monadz.NewWriter(7, "seven").AndThen(func(n int) monadz.Writer[string, int] { return monadz.NewWriter(n+1, "eight") }),
		}
		for _, a := range eitLawInts {
			t.Run(fmt.Sprintf("When left identity is checked for a=%d", a), func(t *testing.T) {
				t.Run("Then unit(a).AndThen(f) gives the same value and log as f(a)", func(t *testing.T) {
					eitSameWriter(t, unit(a).AndThen(f), f(a))
				})
			})
		}
		for name, m := range ms {
			t.Run("When right identity and associativity are checked for m="+name, func(t *testing.T) {
				t.Run("Then m.AndThen(unit) gives the same value and log as m", func(t *testing.T) {
					eitSameWriter(t, m.AndThen(unit), m)
				})
				t.Run("Then the grouping of AndThen does not change the value or the log", func(t *testing.T) {
					lhs := m.AndThen(f).AndThen(g)
					rhs := m.AndThen(func(x int) monadz.Writer[string, string] { return f(x).AndThen(g) })
					eitSameWriter(t, lhs, rhs)
				})
			})
		}
	})
}
