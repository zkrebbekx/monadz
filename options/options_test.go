package options_test

import (
	"errors"
	"iter"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/zkrebbekx/monadz"
	"github.com/zkrebbekx/monadz/options"
)

var errOptTest = errors.New("options test error")

// optCounted returns a sequence over items and a pointer to the number of
// items that the sequence produced.
func optCounted[T any](items ...T) (iter.Seq[T], *int) {
	n := 0
	return func(yield func(T) bool) {
		for _, v := range items {
			n++
			if !yield(v) {
				return
			}
		}
	}, &n
}

func optHalf(n int) monadz.Option[int] { return monadz.When(n%2 == 0, n/2) }

func TestZipAndMap3(t *testing.T) {
	t.Run("Given Some and None Options", func(t *testing.T) {
		a, b, c := monadz.Some(1), monadz.Some("x"), monadz.Some(true)
		noneS := monadz.None[string]()
		t.Run("When Zip is called", func(t *testing.T) {
			t.Run("Then two Somes give Some of a Pair", func(t *testing.T) {
				optEq(t, options.Zip(a, b), monadz.Some(monadz.PairOf(1, "x")))
			})
			t.Run("Then any None gives None", func(t *testing.T) {
				optEq(t, options.Zip(a, noneS).IsNone(), true)
				optEq(t, options.Zip(monadz.None[int](), b).IsNone(), true)
			})
		})
		t.Run("When Zip3 is called", func(t *testing.T) {
			t.Run("Then three Somes give Some of a Triple", func(t *testing.T) {
				optEq(t, options.Zip3(a, b, c), monadz.Some(monadz.TripleOf(1, "x", true)))
			})
			t.Run("Then any None gives None", func(t *testing.T) {
				optEq(t, options.Zip3(a, noneS, c).IsNone(), true)
				optEq(t, options.Zip3(a, b, monadz.None[bool]()).IsNone(), true)
			})
		})
		t.Run("When Map3 is called", func(t *testing.T) {
			calls := 0
			f := func(n int, s string, ok bool) string { calls++; return strconv.Itoa(n) + s + strconv.FormatBool(ok) }
			t.Run("Then three Somes are combined", func(t *testing.T) {
				calls = 0
				optEq(t, options.Map3(a, b, c, f), monadz.Some("1xtrue"))
				optEq(t, calls, 1)
			})
			t.Run("Then any None gives None and f is not called", func(t *testing.T) {
				calls = 0
				optEq(t, options.Map3(monadz.None[int](), b, c, f).IsNone(), true)
				optEq(t, calls, 0)
			})
		})
		t.Run("When Unzip is called", func(t *testing.T) {
			t.Run("Then Some of a Pair gives two Somes", func(t *testing.T) {
				x, y := options.Unzip(monadz.Some(monadz.PairOf(1, "x")))
				optEq(t, x, monadz.Some(1))
				optEq(t, y, monadz.Some("x"))
			})
			t.Run("Then None gives two Nones", func(t *testing.T) {
				x, y := options.Unzip(monadz.None[monadz.Pair[int, string]]())
				optEq(t, x.IsNone(), true)
				optEq(t, y.IsNone(), true)
			})
		})
	})
}

func TestFlattenAndTranspose(t *testing.T) {
	t.Run("Given nested Options", func(t *testing.T) {
		t.Run("When Flatten is called", func(t *testing.T) {
			t.Run("Then Some(Some(v)) gives Some(v)", func(t *testing.T) {
				optEq(t, options.Flatten(monadz.Some(monadz.Some(3))), monadz.Some(3))
			})
			t.Run("Then Some(None) and None give None", func(t *testing.T) {
				optEq(t, options.Flatten(monadz.Some(monadz.None[int]())).IsNone(), true)
				optEq(t, options.Flatten(monadz.None[monadz.Option[int]]()).IsNone(), true)
			})
		})
		t.Run("When Transpose is called", func(t *testing.T) {
			t.Run("Then None gives Ok(None)", func(t *testing.T) {
				r := options.Transpose(monadz.None[monadz.Result[int]]())
				v, err := r.Get()
				optNil(t, err)
				optEq(t, v.IsNone(), true)
			})
			t.Run("Then Some(Ok(v)) gives Ok(Some(v))", func(t *testing.T) {
				v, err := options.Transpose(monadz.Some(monadz.Ok(4))).Get()
				optNil(t, err)
				optEq(t, v, monadz.Some(4))
			})
			t.Run("Then Some(Err(e)) gives Err(e)", func(t *testing.T) {
				r := options.Transpose(monadz.Some(monadz.Err[int](errOptTest)))
				optEq(t, r.Err(), errOptTest)
			})
		})
	})
}

func TestCollectAndTraverse(t *testing.T) {
	t.Run("Given sequences of Options", func(t *testing.T) {
		t.Run("When Collect gets only Somes", func(t *testing.T) {
			seq, _ := optCounted(monadz.Some(1), monadz.Some(2), monadz.Some(3))
			t.Run("Then it gives Some of all values", func(t *testing.T) {
				optEq(t, options.Collect(seq), monadz.Some([]int{1, 2, 3}))
			})
		})
		t.Run("When Collect gets an empty sequence", func(t *testing.T) {
			seq, _ := optCounted[monadz.Option[int]]()
			t.Run("Then it gives Some of an empty, non-nil slice", func(t *testing.T) {
				got := options.Collect(seq)
				optEq(t, got.IsSome(), true)
				optNotNil(t, got.Must())
				optEq(t, len(got.Must()), 0)
			})
		})
		t.Run("When Collect gets a None in the second place", func(t *testing.T) {
			seq, n := optCounted(monadz.Some(1), monadz.None[int](), monadz.Some(3), monadz.Some(4))
			got := options.Collect(seq)
			t.Run("Then it gives None and stops reading at the None", func(t *testing.T) {
				optEq(t, got.IsNone(), true)
				optEq(t, *n, 2)
			})
		})
		t.Run("When Traverse applies a half function", func(t *testing.T) {
			t.Run("Then all even inputs give Some of all halves", func(t *testing.T) {
				optEq(t, options.Traverse(slices.Values([]int{2, 4, 6}), optHalf), monadz.Some([]int{1, 2, 3}))
			})
			t.Run("Then an empty input gives Some of an empty slice", func(t *testing.T) {
				got := options.Traverse(slices.Values([]int{}), optHalf)
				optNotNil(t, got.Must())
				optEq(t, len(got.Must()), 0)
			})
			t.Run("Then an odd input gives None and stops reading", func(t *testing.T) {
				seq, n := optCounted(2, 3, 4, 6)
				optEq(t, options.Traverse(seq, optHalf).IsNone(), true)
				optEq(t, *n, 2)
			})
		})
	})
}

func TestValuesAndFilterMap(t *testing.T) {
	t.Run("Given a mixed sequence of Somes and Nones", func(t *testing.T) {
		items := []monadz.Option[int]{monadz.Some(1), monadz.None[int](), monadz.Some(2), monadz.Some(3)}
		t.Run("When Values is ranged over", func(t *testing.T) {
			t.Run("Then it yields the Some values and skips the Nones", func(t *testing.T) {
				optEq(t, slices.Collect(options.Values(slices.Values(items))), []int{1, 2, 3})
			})
			t.Run("Then a break after the first element stops the source", func(t *testing.T) {
				seq, n := optCounted(items...)
				var got []int
				for v := range options.Values(seq) {
					got = append(got, v)
					break
				}
				optEq(t, got, []int{1})
				optEq(t, *n, 1)
			})
		})
		t.Run("When FilterMap applies a half function", func(t *testing.T) {
			t.Run("Then it yields the halves of the even inputs", func(t *testing.T) {
				optEq(t, slices.Collect(options.FilterMap(slices.Values([]int{1, 2, 3, 4}), optHalf)), []int{1, 2})
			})
			t.Run("Then a break after the first element stops the source", func(t *testing.T) {
				seq, n := optCounted(1, 2, 3, 4, 6)
				var got []int
				for v := range options.FilterMap(seq, optHalf) {
					got = append(got, v)
					break
				}
				optEq(t, got, []int{1})
				optEq(t, *n, 2)
			})
		})
	})
}

func TestFirstComposeEqualContains(t *testing.T) {
	t.Run("Given some Options", func(t *testing.T) {
		none := monadz.None[int]()
		t.Run("When First is called", func(t *testing.T) {
			t.Run("Then it gives the first Some", func(t *testing.T) {
				optEq(t, options.First(none, monadz.Some(2), monadz.Some(3)), monadz.Some(2))
			})
			t.Run("Then only Nones or no arguments give None", func(t *testing.T) {
				optEq(t, options.First(none, none).IsNone(), true)
				optEq(t, options.First[int]().IsNone(), true)
			})
		})
		t.Run("When Compose joins half with half", func(t *testing.T) {
			quarter := options.Compose(optHalf, optHalf)
			t.Run("Then a multiple of four gives Some of a quarter", func(t *testing.T) {
				optEq(t, quarter(8), monadz.Some(2))
			})
			t.Run("Then a failure in either step gives None", func(t *testing.T) {
				optEq(t, quarter(3).IsNone(), true)
				optEq(t, quarter(6).IsNone(), true)
			})
			t.Run("Then the second step is not called when the first fails", func(t *testing.T) {
				called := false
				g := options.Compose(optHalf, func(int) monadz.Option[string] { called = true; return monadz.Some("") })
				optEq(t, g(1).IsNone(), true)
				optEq(t, called, false)
			})
		})
		t.Run("When Equal is called", func(t *testing.T) {
			t.Run("Then equal Somes and two Nones are equal", func(t *testing.T) {
				optEq(t, options.Equal(monadz.Some(1), monadz.Some(1)), true)
				optEq(t, options.Equal(none, none), true)
			})
			t.Run("Then different values and Some(0) against None are not equal", func(t *testing.T) {
				optEq(t, options.Equal(monadz.Some(1), monadz.Some(2)), false)
				optEq(t, options.Equal(monadz.Some(0), none), false)
				optEq(t, options.Equal(none, monadz.Some(0)), false)
			})
		})
		t.Run("When Contains is called", func(t *testing.T) {
			t.Run("Then it is true only for Some of the same value", func(t *testing.T) {
				optEq(t, options.Contains(monadz.Some(1), 1), true)
				optEq(t, options.Contains(monadz.Some(1), 2), false)
				optEq(t, options.Contains(none, 0), false)
			})
		})
	})
}

func TestSequenceQueries(t *testing.T) {
	t.Run("Given sequences of ints", func(t *testing.T) {
		empty := slices.Values([]int{})
		t.Run("When Find is called", func(t *testing.T) {
			t.Run("Then it gives the first match and stops reading", func(t *testing.T) {
				seq, n := optCounted(1, 3, 4, 6)
				optEq(t, options.Find(seq, func(v int) bool { return v%2 == 0 }), monadz.Some(4))
				optEq(t, *n, 3)
			})
			t.Run("Then no match gives None", func(t *testing.T) {
				optEq(t, options.Find(slices.Values([]int{1, 3}), func(v int) bool { return v > 5 }).IsNone(), true)
				optEq(t, options.Find(empty, func(int) bool { return true }).IsNone(), true)
			})
		})
		t.Run("When Head is called", func(t *testing.T) {
			t.Run("Then it gives the first item and reads only one item", func(t *testing.T) {
				seq, n := optCounted(7, 8, 9)
				optEq(t, options.Head(seq), monadz.Some(7))
				optEq(t, *n, 1)
			})
			t.Run("Then an empty sequence gives None", func(t *testing.T) {
				optEq(t, options.Head(empty).IsNone(), true)
			})
		})
		t.Run("When Last is called", func(t *testing.T) {
			t.Run("Then it gives the last item", func(t *testing.T) {
				optEq(t, options.Last(slices.Values([]int{7, 8, 0})), monadz.Some(0))
			})
			t.Run("Then an empty sequence gives None", func(t *testing.T) {
				optEq(t, options.Last(empty).IsNone(), true)
			})
		})
		t.Run("When Max and Min are called", func(t *testing.T) {
			s := slices.Values([]int{3, -1, 9, 4})
			t.Run("Then they give the largest and the smallest item", func(t *testing.T) {
				optEq(t, options.Max(s), monadz.Some(9))
				optEq(t, options.Min(s), monadz.Some(-1))
			})
			t.Run("Then one item gives that item", func(t *testing.T) {
				optEq(t, options.Max(slices.Values([]int{0})), monadz.Some(0))
				optEq(t, options.Min(slices.Values([]int{0})), monadz.Some(0))
			})
			t.Run("Then strings compare in order", func(t *testing.T) {
				optEq(t, options.Max(slices.Values([]string{"b", "c", "a"})), monadz.Some("c"))
				optEq(t, options.Min(slices.Values([]string{"b", "c", "a"})), monadz.Some("a"))
			})
			t.Run("Then an empty sequence gives None, with no panic", func(t *testing.T) {
				optEq(t, options.Max(empty).IsNone(), true)
				optEq(t, options.Min(empty).IsNone(), true)
			})
		})
	})
}

// optEq fails the test when got and want are not deeply equal.
func optEq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// optIsNil reports whether v is nil, or a typed nil pointer, slice, map,
// func, chan, or interface.
func optIsNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan, reflect.Interface:
		return rv.IsNil()
	}
	return false
}

// optNil fails the test when v is not nil.
func optNil(t *testing.T, v any) {
	t.Helper()
	if !optIsNil(v) {
		t.Errorf("got %#v, want nil", v)
	}
}

// optNotNil fails the test when v is nil.
func optNotNil(t *testing.T, v any) {
	t.Helper()
	if optIsNil(v) {
		t.Errorf("got nil, want a non-nil value")
	}
}
