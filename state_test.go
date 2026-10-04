package monadz_test

import (
	"fmt"
	"testing"

	"github.com/zkrebbekx/monadz"
)

// eitBucket is the state of a token bucket rate limiter.
type eitBucket struct {
	Tokens   int
	Capacity int
}

// eitRefill adds n tokens, up to the capacity.
func eitRefill(n int) monadz.State[eitBucket, monadz.Unit] {
	return monadz.ModifyState(func(b eitBucket) eitBucket {
		b.Tokens = min(b.Tokens+n, b.Capacity)
		return b
	})
}

// eitTake takes one token if there is one, and reports whether it did.
func eitTake() monadz.State[eitBucket, bool] {
	return monadz.NewState(func(b eitBucket) (bool, eitBucket) {
		if b.Tokens == 0 {
			return false, b
		}
		b.Tokens--
		return true, b
	})
}

// eitNextLine returns the next invoice line and increments the line counter.
func eitNextLine(desc string) monadz.State[int, string] {
	return monadz.GetState[int]().AndThen(func(n int) monadz.State[int, string] {
		return monadz.PutState(n + 1).Then(monadz.StateOf[int](fmt.Sprintf("%03d %s", n, desc)))
	})
}

func TestStatePrimitives(t *testing.T) {
	t.Run("Given a start state of 5", func(t *testing.T) {
		const start = 5
		t.Run("When StateOf runs", func(t *testing.T) {
			a, s := monadz.StateOf[int]("v").Run(start)
			t.Run("Then it returns the value and keeps the state", func(t *testing.T) {
				eitEq(t, a, "v")
				eitEq(t, s, 5)
			})
		})
		t.Run("When GetState runs", func(t *testing.T) {
			a, s := monadz.GetState[int]().Run(start)
			t.Run("Then the value is the state", func(t *testing.T) {
				eitEq(t, a, 5)
				eitEq(t, s, 5)
			})
		})
		t.Run("When PutState(9) runs", func(t *testing.T) {
			a, s := monadz.PutState(9).Run(start)
			t.Run("Then the state is replaced and the value is Unit", func(t *testing.T) {
				eitEq(t, a, monadz.Unit{})
				eitEq(t, s, 9)
			})
		})
		t.Run("When ModifyState doubles the state", func(t *testing.T) {
			s := monadz.ModifyState(eitDouble).Exec(start)
			t.Run("Then the state is 10", func(t *testing.T) {
				eitEq(t, s, 10)
			})
		})
		t.Run("When GetsState reads a projection", func(t *testing.T) {
			a, s := monadz.GetsState(func(n int) string { return fmt.Sprint(n * n) }).Run(start)
			t.Run("Then the value is the projection and the state does not change", func(t *testing.T) {
				eitEq(t, a, "25")
				eitEq(t, s, 5)
			})
		})
		t.Run("When NewState wraps a function and Eval and Exec run it", func(t *testing.T) {
			m := monadz.NewState(func(s int) (string, int) { return fmt.Sprint(s), s + 1 })
			t.Run("Then Eval returns only the value and Exec only the state", func(t *testing.T) {
				eitEq(t, m.Eval(start), "5")
				eitEq(t, m.Exec(start), 6)
			})
		})
	})
}

func TestStateCombinators(t *testing.T) {
	t.Run("Given a step that returns the state and increments it", func(t *testing.T) {
		tick := monadz.NewState(func(s int) (int, int) { return s, s + 1 })
		t.Run("When Map formats the value", func(t *testing.T) {
			a, s := tick.Map(func(n int) string { return fmt.Sprintf("#%d", n) }).Run(1)
			t.Run("Then the value is mapped and the state still advances", func(t *testing.T) {
				eitEq(t, a, "#1")
				eitEq(t, s, 2)
			})
		})
		t.Run("When AndThen chains two ticks", func(t *testing.T) {
			m := tick.AndThen(func(a int) monadz.State[int, [2]int] {
				return tick.Map(func(b int) [2]int { return [2]int{a, b} })
			})
			a, s := m.Run(10)
			t.Run("Then the second tick sees the state from the first", func(t *testing.T) {
				eitEq(t, a, [2]int{10, 11})
				eitEq(t, s, 12)
			})
		})
		t.Run("When Then runs two ticks", func(t *testing.T) {
			a, s := tick.Then(tick).Run(10)
			t.Run("Then it keeps the second value and the final state", func(t *testing.T) {
				eitEq(t, a, 11)
				eitEq(t, s, 12)
			})
		})
		t.Run("When a State is built and not run", func(t *testing.T) {
			calls := 0
			m := monadz.NewState(func(s int) (int, int) { calls++; return s, s }).Map(eitDouble)
			before := calls
			m.Run(1)
			t.Run("Then nothing runs until Run", func(t *testing.T) {
				eitEq(t, before, 0)
				eitEq(t, calls, 1)
			})
		})
	})
}

func TestStateTokenBucket(t *testing.T) {
	t.Run("Given a token bucket program: take, take, take, refill 2, take", func(t *testing.T) {
		program := eitTake().AndThen(func(a bool) monadz.State[eitBucket, []bool] {
			return eitTake().AndThen(func(b bool) monadz.State[eitBucket, []bool] {
				return eitTake().AndThen(func(c bool) monadz.State[eitBucket, []bool] {
					return eitRefill(2).Then(eitTake().Map(func(d bool) []bool {
						return []bool{a, b, c, d}
					}))
				})
			})
		})
		t.Run("When it runs against a bucket with 2 of 3 tokens", func(t *testing.T) {
			got, end := program.Run(eitBucket{Tokens: 2, Capacity: 3})
			t.Run("Then the third take fails and the refill lets the fourth pass", func(t *testing.T) {
				eitEq(t, got, []bool{true, true, false, true})
				eitEq(t, end, eitBucket{Tokens: 1, Capacity: 3})
			})
		})
		t.Run("When the same program runs against two start states", func(t *testing.T) {
			gotFull, endFull := program.Run(eitBucket{Tokens: 3, Capacity: 3})
			gotEmpty, endEmpty := program.Run(eitBucket{Tokens: 0, Capacity: 1})
			t.Run("Then each run gives an independent result", func(t *testing.T) {
				eitEq(t, gotFull, []bool{true, true, true, true})
				eitEq(t, endFull, eitBucket{Tokens: 1, Capacity: 3})
				eitEq(t, gotEmpty, []bool{false, false, false, true})
				eitEq(t, endEmpty, eitBucket{Tokens: 0, Capacity: 1})
			})
			t.Run("Then a replay gives the same result as the first run", func(t *testing.T) {
				again, againEnd := program.Run(eitBucket{Tokens: 3, Capacity: 3})
				eitEq(t, again, gotFull)
				eitEq(t, againEnd, endFull)
			})
		})
	})
}

func TestStateInvoiceLines(t *testing.T) {
	t.Run("Given an invoice program that numbers three lines", func(t *testing.T) {
		program := eitNextLine("widget").AndThen(func(a string) monadz.State[int, []string] {
			return eitNextLine("gadget").AndThen(func(b string) monadz.State[int, []string] {
				return eitNextLine("shipping").Map(func(c string) []string { return []string{a, b, c} })
			})
		})
		t.Run("When it runs from line 1", func(t *testing.T) {
			lines, next := program.Run(1)
			t.Run("Then the lines are 001, 002, 003 and the next number is 4", func(t *testing.T) {
				eitEq(t, lines, []string{"001 widget", "002 gadget", "003 shipping"})
				eitEq(t, next, 4)
			})
		})
		t.Run("When it runs from line 1 and from line 40", func(t *testing.T) {
			a := program.Eval(1)
			b := program.Eval(40)
			t.Run("Then the runs do not affect each other", func(t *testing.T) {
				eitEq(t, a[0], "001 widget")
				eitEq(t, b, []string{"040 widget", "041 gadget", "042 shipping"})
				eitEq(t, program.Exec(40), 43)
			})
		})
	})
}
