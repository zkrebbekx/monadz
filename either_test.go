package monadz_test

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/zkrebbekx/monadz"
)

// eitEq fails the test if got and want are not deeply equal.
func eitEq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// eitTrue fails the test if cond is false.
func eitTrue(t *testing.T, cond bool, what string) {
	t.Helper()
	if !cond {
		t.Errorf("want %s to be true, got false", what)
	}
}

// eitMustPanic fails the test if f does not panic with want.
func eitMustPanic(t *testing.T, want any, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		got := recover()
		if got == nil {
			t.Errorf("want panic %v, got no panic", want)
			return
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got panic %#v, want panic %#v", got, want)
		}
	}()
	f()
}

// eitNoPanic fails the test if f panics.
func eitNoPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		if r := recover(); r != nil {
			t.Errorf("want no panic, got panic %v", r)
		}
	}()
	f()
}

type eitPaywall struct{ Reason string }

type eitGrant struct{ Plan string }

func eitDouble(n int) int { return n * 2 }

func TestEitherConstructors(t *testing.T) {
	t.Run("Given a Left and a Right Either", func(t *testing.T) {
		l := monadz.Left[string, int]("no")
		r := monadz.Right[string](7)
		t.Run("When the state is inspected", func(t *testing.T) {
			t.Run("Then the Left reports IsLeft and not IsRight", func(t *testing.T) {
				eitTrue(t, l.IsLeft(), "l.IsLeft()")
				eitTrue(t, !l.IsRight(), "!l.IsRight()")
			})
			t.Run("Then the Right reports IsRight and not IsLeft", func(t *testing.T) {
				eitTrue(t, r.IsRight(), "r.IsRight()")
				eitTrue(t, !r.IsLeft(), "!r.IsLeft()")
			})
		})
	})

	t.Run("Given the zero value of Either", func(t *testing.T) {
		var z monadz.Either[string, int]
		t.Run("When it is inspected", func(t *testing.T) {
			t.Run("Then it is Left of the zero L", func(t *testing.T) {
				eitTrue(t, z.IsLeft(), "z.IsLeft()")
				eitEq(t, z.LeftOr("fallback"), "")
				eitEq(t, z.Left(), monadz.Some(""))
				eitTrue(t, z.Right().IsNone(), "z.Right().IsNone()")
				eitEq(t, z.String(), "Left()")
			})
		})
	})
}

func TestEitherPredicates(t *testing.T) {
	t.Run("Given a Left of 3 and a Right of 4", func(t *testing.T) {
		l := monadz.Left[int, int](3)
		r := monadz.Right[int](4)
		even := func(n int) bool { return n%2 == 0 }
		odd := func(n int) bool { return n%2 == 1 }
		t.Run("When IsLeftAnd is called", func(t *testing.T) {
			t.Run("Then it is true only for a Left that satisfies the predicate", func(t *testing.T) {
				eitEq(t, l.IsLeftAnd(odd), true)
				eitEq(t, l.IsLeftAnd(even), false)
				eitEq(t, r.IsLeftAnd(even), false)
			})
		})
		t.Run("When IsRightAnd is called", func(t *testing.T) {
			t.Run("Then it is true only for a Right that satisfies the predicate", func(t *testing.T) {
				eitEq(t, r.IsRightAnd(even), true)
				eitEq(t, r.IsRightAnd(odd), false)
				eitEq(t, l.IsRightAnd(odd), false)
			})
		})
		t.Run("When the predicate panics if called", func(t *testing.T) {
			boom := func(int) bool { panic("must not run") }
			t.Run("Then IsLeftAnd on a Right and IsRightAnd on a Left do not call it", func(t *testing.T) {
				eitNoPanic(t, func() { r.IsLeftAnd(boom) })
				eitNoPanic(t, func() { l.IsRightAnd(boom) })
			})
		})
	})
}

func TestEitherAccessors(t *testing.T) {
	t.Run(`Given a Left of "x" and a Right of 9`, func(t *testing.T) {
		l := monadz.Left[string, int]("x")
		r := monadz.Right[string](9)
		t.Run("When Left() and Right() are called", func(t *testing.T) {
			t.Run("Then each returns Some only for the live side", func(t *testing.T) {
				eitEq(t, l.Left(), monadz.Some("x"))
				eitEq(t, l.Right(), monadz.None[int]())
				eitEq(t, r.Right(), monadz.Some(9))
				eitEq(t, r.Left(), monadz.None[string]())
			})
		})
		t.Run("When Unpack is called", func(t *testing.T) {
			lv, rv, ok := l.Unpack()
			lv2, rv2, ok2 := r.Unpack()
			t.Run("Then it returns both slots and the Right flag", func(t *testing.T) {
				eitEq(t, []any{lv, rv, ok}, []any{"x", 0, false})
				eitEq(t, []any{lv2, rv2, ok2}, []any{"", 9, true})
			})
		})
		t.Run("When LeftOr and RightOr are called", func(t *testing.T) {
			t.Run("Then the live value wins, else the fallback", func(t *testing.T) {
				eitEq(t, l.LeftOr("fb"), "x")
				eitEq(t, r.LeftOr("fb"), "fb")
				eitEq(t, r.RightOr(-1), 9)
				eitEq(t, l.RightOr(-1), -1)
			})
		})
		t.Run("When RightOrElse is called", func(t *testing.T) {
			calls := 0
			f := func(s string) int { calls++; return len(s) * 100 }
			gotL := l.RightOrElse(f)
			gotR := r.RightOrElse(f)
			t.Run("Then a Left goes through f and a Right does not", func(t *testing.T) {
				eitEq(t, gotL, 100)
				eitEq(t, gotR, 9)
				eitEq(t, calls, 1)
			})
		})
	})
}

func TestEitherMap(t *testing.T) {
	t.Run(`Given a Right of 21 and a Left of "bad"`, func(t *testing.T) {
		r := monadz.Right[string](21)
		l := monadz.Left[string, int]("bad")
		t.Run("When Map changes the Right type", func(t *testing.T) {
			got := r.Map(strconv.Itoa)
			t.Run("Then the Right value is mapped", func(t *testing.T) {
				eitEq(t, got, monadz.Right[string]("21"))
			})
		})
		t.Run("When Map is called on a Left", func(t *testing.T) {
			called := false
			got := l.Map(func(int) string { called = true; return "" })
			t.Run("Then f is not called and the Left passes through", func(t *testing.T) {
				eitEq(t, called, false)
				eitEq(t, got, monadz.Left[string, string]("bad"))
			})
		})
		t.Run("When MapLeft is called on both", func(t *testing.T) {
			calls := 0
			f := func(s string) int { calls++; return len(s) }
			gotL := l.MapLeft(f)
			gotR := r.MapLeft(f)
			t.Run("Then only the Left value is mapped", func(t *testing.T) {
				eitEq(t, gotL, monadz.Left[int, int](3))
				eitEq(t, gotR, monadz.Right[int](21))
				eitEq(t, calls, 1)
			})
		})
		t.Run("When BiMap is called on both", func(t *testing.T) {
			leftCalls, rightCalls := 0, 0
			onLeft := func(s string) error { leftCalls++; return fmt.Errorf("e:%s", s) }
			onRight := func(n int) float64 { rightCalls++; return float64(n) / 2 }
			gotL := l.BiMap(onLeft, onRight)
			gotR := r.BiMap(onLeft, onRight)
			t.Run("Then each side calls only its own function", func(t *testing.T) {
				eitTrue(t, gotL.IsLeft(), "gotL.IsLeft()")
				eitEq(t, gotL.LeftOr(nil).Error(), "e:bad")
				eitEq(t, gotR, monadz.Right[error](10.5))
				eitEq(t, leftCalls, 1)
				eitEq(t, rightCalls, 1)
			})
		})
	})
}

func TestEitherAndThenOrElse(t *testing.T) {
	t.Run("Given a parse step that returns Either[string, int]", func(t *testing.T) {
		parse := func(s string) monadz.Either[string, int] {
			n, err := strconv.Atoi(s)
			if err != nil {
				return monadz.Left[string, int]("not a number: " + s)
			}
			return monadz.Right[string](n)
		}
		t.Run("When AndThen runs on a Right that parses", func(t *testing.T) {
			got := monadz.Right[string]("42").AndThen(parse)
			t.Run("Then the result is the Right of the parsed value", func(t *testing.T) {
				eitEq(t, got, monadz.Right[string](42))
			})
		})
		t.Run("When AndThen runs on a Right that does not parse", func(t *testing.T) {
			got := monadz.Right[string]("x").AndThen(parse)
			t.Run("Then the result is the Left from the step", func(t *testing.T) {
				eitEq(t, got, monadz.Left[string, int]("not a number: x"))
			})
		})
		t.Run("When AndThen runs on a Left", func(t *testing.T) {
			called := false
			got := monadz.Left[string, string]("early").AndThen(func(s string) monadz.Either[string, int] {
				called = true
				return parse(s)
			})
			t.Run("Then the step is skipped and the Left passes through", func(t *testing.T) {
				eitEq(t, called, false)
				eitEq(t, got, monadz.Left[string, int]("early"))
			})
		})
		t.Run("When OrElse recovers a Left", func(t *testing.T) {
			got := monadz.Left[string, int]("7").OrElse(func(s string) monadz.Either[error, int] {
				n, err := strconv.Atoi(s)
				if err != nil {
					return monadz.Left[error, int](err)
				}
				return monadz.Right[error](n)
			})
			t.Run("Then the result is the recovered Right with the new L type", func(t *testing.T) {
				eitEq(t, got, monadz.Right[error](7))
			})
		})
		t.Run("When OrElse runs on a Right", func(t *testing.T) {
			called := false
			got := monadz.Right[string](5).OrElse(func(string) monadz.Either[int, int] {
				called = true
				return monadz.Left[int, int](0)
			})
			t.Run("Then f is not called and the Right passes through", func(t *testing.T) {
				eitEq(t, called, false)
				eitEq(t, got, monadz.Right[int](5))
			})
		})
	})
}

func TestEitherMatchSwapString(t *testing.T) {
	t.Run("Given a paywall decision as Either[eitPaywall, eitGrant]", func(t *testing.T) {
		blocked := monadz.Left[eitPaywall, eitGrant](eitPaywall{Reason: "trial over"})
		granted := monadz.Right[eitPaywall](eitGrant{Plan: "pro"})
		render := func(e monadz.Either[eitPaywall, eitGrant]) string {
			return e.Match(
				func(p eitPaywall) string { return "paywall: " + p.Reason },
				func(g eitGrant) string { return "welcome, " + g.Plan },
			)
		}
		t.Run("When Match is called", func(t *testing.T) {
			t.Run("Then each side calls its own branch", func(t *testing.T) {
				eitEq(t, render(blocked), "paywall: trial over")
				eitEq(t, render(granted), "welcome, pro")
			})
		})
		t.Run("When Swap is called", func(t *testing.T) {
			sb := blocked.Swap()
			sg := granted.Swap()
			t.Run("Then the sides are exchanged, and a second Swap restores them", func(t *testing.T) {
				eitEq(t, sb, monadz.Right[eitGrant](eitPaywall{Reason: "trial over"}))
				eitEq(t, sg, monadz.Left[eitGrant, eitPaywall](eitGrant{Plan: "pro"}))
				eitEq(t, sb.Swap(), blocked)
			})
		})
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then it names the side and formats the value", func(t *testing.T) {
				eitEq(t, blocked.String(), "Left({trial over})")
				eitEq(t, granted.String(), "Right({pro})")
				eitEq(t, fmt.Sprint(monadz.Right[string](eitDouble(2))), "Right(4)")
			})
		})
	})
}
