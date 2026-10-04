package eithers_test

import (
	"errors"
	"iter"
	"reflect"
	"slices"
	"testing"

	"github.com/zkrebbekx/lift"
	"github.com/zkrebbekx/lift/eithers"
)

// eitEq fails the test if got and want are not deeply equal.
func eitEq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// eitCounted returns a sequence over es and a counter of the values that
// the sequence yielded.
func eitCounted[L, R any](es ...lift.Either[L, R]) (iter.Seq[lift.Either[L, R]], *int) {
	n := 0
	return func(yield func(lift.Either[L, R]) bool) {
		for _, e := range es {
			n++
			if !yield(e) {
				return
			}
		}
	}, &n
}

func eitL(s string) lift.Either[string, int] { return lift.Left[string, int](s) }
func eitR(n int) lift.Either[string, int]    { return lift.Right[string](n) }

func TestPartition(t *testing.T) {
	t.Run("Given a mixed sequence of Lefts and Rights", func(t *testing.T) {
		seq := slices.Values([]lift.Either[string, int]{eitR(1), eitL("a"), eitR(2), eitL("b"), eitR(3)})
		t.Run("When Partition runs", func(t *testing.T) {
			ls, rs := eithers.Partition(seq)
			t.Run("Then it splits the values and keeps the order", func(t *testing.T) {
				eitEq(t, ls, []string{"a", "b"})
				eitEq(t, rs, []int{1, 2, 3})
			})
		})
	})
	t.Run("Given an empty sequence", func(t *testing.T) {
		t.Run("When Partition runs", func(t *testing.T) {
			ls, rs := eithers.Partition(slices.Values([]lift.Either[string, int](nil)))
			t.Run("Then both slices are empty", func(t *testing.T) {
				eitEq(t, len(ls)+len(rs), 0)
			})
		})
	})
}

func TestLeftsRights(t *testing.T) {
	t.Run("Given a mixed sequence of Lefts and Rights", func(t *testing.T) {
		all := []lift.Either[string, int]{eitL("a"), eitR(1), eitL("b"), eitR(2), eitL("c"), eitR(3)}
		t.Run("When Lefts and Rights are collected", func(t *testing.T) {
			ls := slices.Collect(eithers.Lefts(slices.Values(all)))
			rs := slices.Collect(eithers.Rights(slices.Values(all)))
			t.Run("Then each yields only its own side, in order", func(t *testing.T) {
				eitEq(t, ls, []string{"a", "b", "c"})
				eitEq(t, rs, []int{1, 2, 3})
			})
		})
		t.Run("When the loop over Lefts breaks after the first value", func(t *testing.T) {
			seq, pulled := eitCounted(all...)
			var got []string
			for l := range eithers.Lefts(seq) {
				got = append(got, l)
				break
			}
			t.Run("Then the source stops after the first element", func(t *testing.T) {
				eitEq(t, got, []string{"a"})
				eitEq(t, *pulled, 1)
			})
		})
		t.Run("When the loop over Rights breaks after the second value", func(t *testing.T) {
			seq, pulled := eitCounted(all...)
			var got []int
			for r := range eithers.Rights(seq) {
				got = append(got, r)
				if len(got) == 2 {
					break
				}
			}
			t.Run("Then the source stops at the element that gave the second value", func(t *testing.T) {
				eitEq(t, got, []int{1, 2})
				eitEq(t, *pulled, 4)
			})
		})
	})
}

func TestFlatten(t *testing.T) {
	t.Run("Given nested Eithers", func(t *testing.T) {
		outerLeft := lift.Left[string, lift.Either[string, int]]("outer")
		innerLeft := lift.Right[string](eitL("inner"))
		innerRight := lift.Right[string](eitR(5))
		t.Run("When Flatten runs", func(t *testing.T) {
			t.Run("Then an outer Left stays, and an outer Right gives the inner Either", func(t *testing.T) {
				eitEq(t, eithers.Flatten(outerLeft), eitL("outer"))
				eitEq(t, eithers.Flatten(innerLeft), eitL("inner"))
				eitEq(t, eithers.Flatten(innerRight), eitR(5))
			})
		})
	})
}

func TestToResult(t *testing.T) {
	errBad := errors.New("bad")
	t.Run("Given Either[error, int] values", func(t *testing.T) {
		t.Run("When ToResult converts a Right", func(t *testing.T) {
			got := eithers.ToResult(lift.Right[error](4))
			t.Run("Then the result is Ok", func(t *testing.T) {
				eitEq(t, got, lift.Ok(4))
			})
		})
		t.Run("When ToResult converts a Left", func(t *testing.T) {
			got := eithers.ToResult(lift.Left[error, int](errBad))
			t.Run("Then the result is Err with the same error", func(t *testing.T) {
				if !errors.Is(got.Err(), errBad) {
					t.Errorf("got err %v, want %v", got.Err(), errBad)
				}
			})
		})
		t.Run("When ToResult converts a Left of a nil error (the zero Either)", func(t *testing.T) {
			var zero lift.Either[error, int]
			got := eithers.ToResult(zero)
			t.Run("Then the result is Err(ErrNilError), not a success", func(t *testing.T) {
				if !errors.Is(got.Err(), lift.ErrNilError) {
					t.Errorf("got err %v, want %v", got.Err(), lift.ErrNilError)
				}
			})
		})
	})
}

func TestCollect(t *testing.T) {
	t.Run("Given a sequence with only Rights", func(t *testing.T) {
		t.Run("When Collect runs", func(t *testing.T) {
			got := eithers.Collect(slices.Values([]lift.Either[string, int]{eitR(1), eitR(2)}))
			t.Run("Then it returns Right of all values", func(t *testing.T) {
				eitEq(t, got, lift.Right[string]([]int{1, 2}))
			})
		})
	})
	t.Run("Given an empty sequence", func(t *testing.T) {
		t.Run("When Collect runs", func(t *testing.T) {
			got := eithers.Collect(slices.Values([]lift.Either[string, int](nil)))
			t.Run("Then it returns Right of an empty, non-nil slice", func(t *testing.T) {
				eitEq(t, got, lift.Right[string]([]int{}))
			})
		})
	})
	t.Run("Given a sequence with a Left in the middle", func(t *testing.T) {
		seq, pulled := eitCounted(eitR(1), eitL("first"), eitR(2), eitL("second"))
		t.Run("When Collect runs", func(t *testing.T) {
			got := eithers.Collect(seq)
			t.Run("Then it returns the first Left and stops the source there", func(t *testing.T) {
				eitEq(t, got, lift.Left[string, []int]("first"))
				eitEq(t, *pulled, 2)
			})
		})
	})
}
