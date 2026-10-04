package monadz_test

import (
	"fmt"
	"testing"

	"github.com/zkrebbekx/monadz"
)

func TestPair(t *testing.T) {
	t.Run("Given PairOf(1, \"a\")", func(t *testing.T) {
		p := monadz.PairOf(1, "a")
		t.Run("When the fields are read", func(t *testing.T) {
			t.Run("Then First and Second hold the values", func(t *testing.T) {
				wantDeep(t, p.First, 1)
				wantDeep(t, p.Second, "a")
			})
		})
		t.Run("When Unpack is called", func(t *testing.T) {
			a, b := p.Unpack()
			t.Run("Then it returns both values in order", func(t *testing.T) {
				wantDeep(t, a, 1)
				wantDeep(t, b, "a")
			})
		})
		t.Run("When Swap is called", func(t *testing.T) {
			s := p.Swap()
			t.Run("Then the values change order", func(t *testing.T) {
				wantDeep(t, s, monadz.Pair[string, int]{First: "a", Second: 1})
				wantDeep(t, s.Swap(), p)
			})
		})
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then it gives (a, b)", func(t *testing.T) {
				wantDeep(t, p.String(), "(1, a)")
				wantDeep(t, fmt.Sprint(p), "(1, a)")
			})
		})
	})
	t.Run("Given TripleOf(1, \"b\", true)", func(t *testing.T) {
		tr := monadz.TripleOf(1, "b", true)
		t.Run("When Unpack is called", func(t *testing.T) {
			a, b, c := tr.Unpack()
			t.Run("Then it returns the three values in order", func(t *testing.T) {
				wantDeep(t, a, 1)
				wantDeep(t, b, "b")
				wantDeep(t, c, true)
				wantDeep(t, tr, monadz.Triple[int, string, bool]{First: 1, Second: "b", Third: true})
			})
		})
	})
	t.Run("Given the Unit type", func(t *testing.T) {
		t.Run("When two Unit values are compared", func(t *testing.T) {
			t.Run("Then they are equal", func(t *testing.T) {
				wantDeep(t, monadz.Unit{} == monadz.Unit{}, true)
			})
		})
	})
}

func TestTripleString(t *testing.T) {
	t.Run("Given a Triple of an int, a string, and a bool", func(t *testing.T) {
		tr := monadz.TripleOf(1, "a", true)
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then it returns (a, b, c)", func(t *testing.T) {
				wantEq(t, tr.String(), "(1, a, true)")
				wantEq(t, fmt.Sprint(tr), "(1, a, true)")
			})
		})
	})
}
