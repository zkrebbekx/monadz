package monadz_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/zkrebbekx/monadz"
)

var optErrTest = errors.New("opt test error")

func optIsEven(n int) bool { return n%2 == 0 }

func optHalf(n int) monadz.Option[int] {
	if n%2 != 0 {
		return monadz.None[int]()
	}
	return monadz.Some(n / 2)
}

func optAtoi(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func TestOptionConstructors(t *testing.T) {
	t.Run("Given the Option constructors", func(t *testing.T) {
		t.Run("When Some(3) is built", func(t *testing.T) {
			o := monadz.Some(3)
			t.Run("Then it is Some and holds 3", func(t *testing.T) {
				wantDeep(t, o.IsSome(), true)
				wantDeep(t, o.IsNone(), false)
				v, ok := o.Get()
				wantDeep(t, v, 3)
				wantDeep(t, ok, true)
			})
		})
		t.Run("When None is built", func(t *testing.T) {
			o := monadz.None[int]()
			t.Run("Then it is None and Get gives the zero value", func(t *testing.T) {
				wantDeep(t, o.IsNone(), true)
				wantDeep(t, o.IsSome(), false)
				v, ok := o.Get()
				wantDeep(t, v, 0)
				wantDeep(t, ok, false)
			})
		})
		t.Run("When the zero value of Option is used", func(t *testing.T) {
			var o monadz.Option[string]
			t.Run("Then it is None", func(t *testing.T) {
				wantDeep(t, o.IsNone(), true)
				wantDeep(t, o, monadz.None[string]())
			})
		})
		t.Run("When Some holds a zero value", func(t *testing.T) {
			o := monadz.Some(0)
			t.Run("Then it is still Some", func(t *testing.T) {
				wantDeep(t, o.IsSome(), true)
			})
		})
		t.Run("When FromTuple is called", func(t *testing.T) {
			t.Run("Then ok=true gives Some and ok=false gives None", func(t *testing.T) {
				wantDeep(t, monadz.FromTuple(5, true), monadz.Some(5))
				wantDeep(t, monadz.FromTuple(5, false), monadz.None[int]())
			})
		})
		t.Run("When FromPtr is called", func(t *testing.T) {
			n := 7
			t.Run("Then a nil pointer gives None", func(t *testing.T) {
				wantDeep(t, monadz.FromPtr[int](nil).IsNone(), true)
			})
			t.Run("Then a non-nil pointer gives Some of a copy", func(t *testing.T) {
				o := monadz.FromPtr(&n)
				n = 8
				wantDeep(t, o, monadz.Some(7))
			})
		})
		t.Run("When FromZero is called", func(t *testing.T) {
			t.Run("Then the zero value gives None", func(t *testing.T) {
				wantDeep(t, monadz.FromZero(0).IsNone(), true)
				wantDeep(t, monadz.FromZero("").IsNone(), true)
				wantDeep(t, monadz.FromZero(false).IsNone(), true)
			})
			t.Run("Then a non-zero value gives Some", func(t *testing.T) {
				wantDeep(t, monadz.FromZero(4), monadz.Some(4))
				wantDeep(t, monadz.FromZero("x"), monadz.Some("x"))
			})
		})
		t.Run("When When is called", func(t *testing.T) {
			t.Run("Then true gives Some and false gives None", func(t *testing.T) {
				wantDeep(t, monadz.When(true, 1), monadz.Some(1))
				wantDeep(t, monadz.When(false, 1).IsNone(), true)
			})
		})
		t.Run("When WhenFunc is called", func(t *testing.T) {
			calls := 0
			f := func() int { calls++; return 9 }
			t.Run("Then true calls f once and gives Some", func(t *testing.T) {
				calls = 0
				wantDeep(t, monadz.WhenFunc(true, f), monadz.Some(9))
				wantDeep(t, calls, 1)
			})
			t.Run("Then false does not call f and gives None", func(t *testing.T) {
				calls = 0
				wantDeep(t, monadz.WhenFunc(false, f).IsNone(), true)
				wantDeep(t, calls, 0)
			})
		})
		t.Run("When Lookup is called on a map", func(t *testing.T) {
			m := map[string]int{"a": 1, "z": 0}
			t.Run("Then a present key gives Some, also for a zero value", func(t *testing.T) {
				wantDeep(t, monadz.Lookup(m, "a"), monadz.Some(1))
				wantDeep(t, monadz.Lookup(m, "z"), monadz.Some(0))
			})
			t.Run("Then an absent key gives None", func(t *testing.T) {
				wantDeep(t, monadz.Lookup(m, "b").IsNone(), true)
			})
			t.Run("Then a nil map gives None", func(t *testing.T) {
				wantDeep(t, monadz.Lookup(map[string]int(nil), "a").IsNone(), true)
			})
		})
		t.Run("When At is called on a slice of three items", func(t *testing.T) {
			s := []string{"a", "b", "c"}
			t.Run("Then an index in range gives Some", func(t *testing.T) {
				wantDeep(t, monadz.At(s, 0), monadz.Some("a"))
				wantDeep(t, monadz.At(s, 2), monadz.Some("c"))
			})
			t.Run("Then a negative index gives None and does not count from the end", func(t *testing.T) {
				wantDeep(t, monadz.At(s, -1), monadz.None[string]())
				wantDeep(t, monadz.At(s, -3), monadz.None[string]())
			})
			t.Run("Then an index out of range gives None", func(t *testing.T) {
				wantDeep(t, monadz.At(s, 3).IsNone(), true)
				wantDeep(t, monadz.At(s, -4).IsNone(), true)
				wantDeep(t, monadz.At([]string(nil), 0).IsNone(), true)
				wantDeep(t, monadz.At([]string(nil), -1).IsNone(), true)
			})
		})
	})
}

func TestOptionQueries(t *testing.T) {
	t.Run("Given Some(4), Some(3), and None", func(t *testing.T) {
		some4, some3, none := monadz.Some(4), monadz.Some(3), monadz.None[int]()
		t.Run("When IsSomeAnd is called with an even test", func(t *testing.T) {
			t.Run("Then only Some of an even value is true", func(t *testing.T) {
				wantDeep(t, some4.IsSomeAnd(optIsEven), true)
				wantDeep(t, some3.IsSomeAnd(optIsEven), false)
				wantDeep(t, none.IsSomeAnd(optIsEven), false)
			})
			t.Run("Then the predicate is not called for None", func(t *testing.T) {
				called := false
				none.IsSomeAnd(func(int) bool { called = true; return true })
				wantDeep(t, called, false)
			})
		})
		t.Run("When IsNoneOr is called with an even test", func(t *testing.T) {
			t.Run("Then None and Some of an even value are true", func(t *testing.T) {
				wantDeep(t, some4.IsNoneOr(optIsEven), true)
				wantDeep(t, some3.IsNoneOr(optIsEven), false)
				wantDeep(t, none.IsNoneOr(optIsEven), true)
			})
			t.Run("Then the predicate is not called for None", func(t *testing.T) {
				called := false
				none.IsNoneOr(func(int) bool { called = true; return false })
				wantDeep(t, called, false)
			})
		})
	})
}

func TestOptionExtraction(t *testing.T) {
	t.Run("Given Some(5) and None", func(t *testing.T) {
		some, none := monadz.Some(5), monadz.None[int]()
		t.Run("When Must is called", func(t *testing.T) {
			t.Run("Then Some returns the value", func(t *testing.T) {
				wantDeep(t, some.Must(), 5)
			})
			t.Run("Then None panics with ErrNone", func(t *testing.T) {
				wantDeep(t, mustPanic(t, func() { none.Must() }), any(monadz.ErrNone))
			})
		})
		t.Run("When Expect is called", func(t *testing.T) {
			t.Run("Then Some returns the value", func(t *testing.T) {
				wantDeep(t, some.Expect("need it"), 5)
			})
			t.Run("Then None panics with an error that wraps ErrNone and holds the message", func(t *testing.T) {
				var got any
				func() {
					defer func() { got = recover() }()
					none.Expect("need port")
				}()
				err, ok := got.(error)
				wantDeep(t, ok, true)
				wantDeep(t, errors.Is(err, monadz.ErrNone), true)
				wantDeep(t, strings.HasPrefix(err.Error(), "need port: "), true)
			})
		})
		t.Run("When ValueOr is called", func(t *testing.T) {
			t.Run("Then Some returns the value and None returns the fallback", func(t *testing.T) {
				wantDeep(t, some.ValueOr(9), 5)
				wantDeep(t, none.ValueOr(9), 9)
			})
		})
		t.Run("When ValueOrElse is called", func(t *testing.T) {
			calls := 0
			f := func() int { calls++; return 9 }
			t.Run("Then Some does not call the fallback", func(t *testing.T) {
				calls = 0
				wantDeep(t, some.ValueOrElse(f), 5)
				wantDeep(t, calls, 0)
			})
			t.Run("Then None calls the fallback once", func(t *testing.T) {
				calls = 0
				wantDeep(t, none.ValueOrElse(f), 9)
				wantDeep(t, calls, 1)
			})
		})
		t.Run("When ValueOrZero is called", func(t *testing.T) {
			t.Run("Then Some returns the value and None returns zero", func(t *testing.T) {
				wantDeep(t, some.ValueOrZero(), 5)
				wantDeep(t, none.ValueOrZero(), 0)
			})
		})
		t.Run("When Ptr is called", func(t *testing.T) {
			t.Run("Then None gives nil", func(t *testing.T) {
				wantNil(t, none.Ptr())
			})
			t.Run("Then Some gives a pointer to a copy", func(t *testing.T) {
				p := some.Ptr()
				wantDeep(t, *p, 5)
				*p = 6
				wantDeep(t, some.Must(), 5)
			})
		})
		t.Run("When Try is called", func(t *testing.T) {
			t.Run("Then Some returns the value", func(t *testing.T) {
				wantDeep(t, some.Try(), 5)
			})
			t.Run("Then None inside Do gives Err(ErrNone)", func(t *testing.T) {
				r := monadz.Do(func() int { return none.Try() + 1 })
				wantDeep(t, errors.Is(r.Err(), monadz.ErrNone), true)
			})
			t.Run("Then None inside DoOption gives None", func(t *testing.T) {
				o := monadz.DoOption(func() int { return none.Try() + 1 })
				wantDeep(t, o.IsNone(), true)
			})
			t.Run("Then Some inside DoOption gives Some", func(t *testing.T) {
				o := monadz.DoOption(func() int { return some.Try() + 1 })
				wantDeep(t, o, monadz.Some(6))
			})
			t.Run("Then None outside Do panics", func(t *testing.T) {
				mustPanic(t, func() { none.Try() })
			})
		})
	})
}

func TestOptionTransformation(t *testing.T) {
	t.Run("Given Some(4) and None", func(t *testing.T) {
		some, none := monadz.Some(4), monadz.None[int]()
		t.Run("When Map changes the type to string", func(t *testing.T) {
			t.Run("Then Some maps and None stays None", func(t *testing.T) {
				wantDeep(t, some.Map(strconv.Itoa), monadz.Some("4"))
				wantDeep(t, none.Map(strconv.Itoa), monadz.None[string]())
			})
			t.Run("Then f is not called for None", func(t *testing.T) {
				called := false
				none.Map(func(int) int { called = true; return 0 })
				wantDeep(t, called, false)
			})
		})
		t.Run("When AndThen is called with a half function", func(t *testing.T) {
			t.Run("Then Some of an even value gives Some of half", func(t *testing.T) {
				wantDeep(t, some.AndThen(optHalf), monadz.Some(2))
			})
			t.Run("Then Some of an odd value gives None", func(t *testing.T) {
				wantDeep(t, monadz.Some(3).AndThen(optHalf).IsNone(), true)
			})
			t.Run("Then None gives None", func(t *testing.T) {
				wantDeep(t, none.AndThen(optHalf).IsNone(), true)
			})
		})
		t.Run("When Then is called with a comma-ok function", func(t *testing.T) {
			t.Run("Then a good string gives Some and a bad string gives None", func(t *testing.T) {
				wantDeep(t, monadz.Some("12").Then(optAtoi), monadz.Some(12))
				wantDeep(t, monadz.Some("x").Then(optAtoi).IsNone(), true)
				wantDeep(t, monadz.None[string]().Then(optAtoi).IsNone(), true)
			})
			t.Run("Then a map lookup plugs in", func(t *testing.T) {
				m := map[int]string{4: "four"}
				lookup := func(k int) (string, bool) { v, ok := m[k]; return v, ok }
				wantDeep(t, some.Then(lookup), monadz.Some("four"))
			})
		})
		t.Run("When MapOr is called", func(t *testing.T) {
			t.Run("Then Some maps and None gives the fallback", func(t *testing.T) {
				wantDeep(t, some.MapOr("none", strconv.Itoa), "4")
				wantDeep(t, none.MapOr("none", strconv.Itoa), "none")
			})
		})
		t.Run("When MapOrElse is called", func(t *testing.T) {
			calls := 0
			fb := func() string { calls++; return "none" }
			t.Run("Then Some maps and does not call the fallback", func(t *testing.T) {
				calls = 0
				wantDeep(t, some.MapOrElse(fb, strconv.Itoa), "4")
				wantDeep(t, calls, 0)
			})
			t.Run("Then None calls the fallback", func(t *testing.T) {
				calls = 0
				wantDeep(t, none.MapOrElse(fb, strconv.Itoa), "none")
				wantDeep(t, calls, 1)
			})
		})
		t.Run("When Match is called", func(t *testing.T) {
			onSome := func(v int) string { return "some " + strconv.Itoa(v) }
			onNone := func() string { return "none" }
			t.Run("Then Some calls onSome and None calls onNone", func(t *testing.T) {
				wantDeep(t, some.Match(onSome, onNone), "some 4")
				wantDeep(t, none.Match(onSome, onNone), "none")
			})
		})
		t.Run("When Filter is called with an even test", func(t *testing.T) {
			t.Run("Then a passing value stays and a failing value gives None", func(t *testing.T) {
				wantDeep(t, some.Filter(optIsEven), monadz.Some(4))
				wantDeep(t, monadz.Some(3).Filter(optIsEven).IsNone(), true)
				wantDeep(t, none.Filter(optIsEven).IsNone(), true)
			})
		})
		t.Run("When Inspect is called", func(t *testing.T) {
			var seen []int
			f := func(v int) { seen = append(seen, v) }
			t.Run("Then Some calls f and returns the Option unchanged", func(t *testing.T) {
				seen = nil
				wantDeep(t, some.Inspect(f), some)
				wantDeep(t, seen, []int{4})
			})
			t.Run("Then None does not call f", func(t *testing.T) {
				seen = nil
				wantDeep(t, none.Inspect(f).IsNone(), true)
				wantDeep(t, len(seen), 0)
			})
		})
	})
}

func TestOptionCombination(t *testing.T) {
	t.Run("Given Some(1), Some(2), and None", func(t *testing.T) {
		one, two, none := monadz.Some(1), monadz.Some(2), monadz.None[int]()
		t.Run("When And is called", func(t *testing.T) {
			t.Run("Then Some gives the other Option and None gives None", func(t *testing.T) {
				wantDeep(t, one.And(monadz.Some("x")), monadz.Some("x"))
				wantDeep(t, one.And(monadz.None[string]()).IsNone(), true)
				wantDeep(t, none.And(monadz.Some("x")).IsNone(), true)
			})
		})
		t.Run("When Or is called", func(t *testing.T) {
			t.Run("Then the first Some wins", func(t *testing.T) {
				wantDeep(t, one.Or(two), one)
				wantDeep(t, none.Or(two), two)
				wantDeep(t, none.Or(none).IsNone(), true)
			})
		})
		t.Run("When OrElse is called", func(t *testing.T) {
			calls := 0
			f := func() monadz.Option[int] { calls++; return two }
			t.Run("Then Some does not call f", func(t *testing.T) {
				calls = 0
				wantDeep(t, one.OrElse(f), one)
				wantDeep(t, calls, 0)
			})
			t.Run("Then None calls f", func(t *testing.T) {
				calls = 0
				wantDeep(t, none.OrElse(f), two)
				wantDeep(t, calls, 1)
			})
		})
		t.Run("When Xor is called", func(t *testing.T) {
			t.Run("Then exactly one Some gives that Some", func(t *testing.T) {
				wantDeep(t, one.Xor(none), one)
				wantDeep(t, none.Xor(two), two)
			})
			t.Run("Then two Somes or two Nones give None", func(t *testing.T) {
				wantDeep(t, one.Xor(two).IsNone(), true)
				wantDeep(t, none.Xor(none).IsNone(), true)
			})
		})
		t.Run("When ZipWith is called with a sum function", func(t *testing.T) {
			sum := func(a int, b string) string { return strconv.Itoa(a) + b }
			t.Run("Then two Somes are combined", func(t *testing.T) {
				wantDeep(t, one.ZipWith(monadz.Some("a"), sum), monadz.Some("1a"))
			})
			t.Run("Then any None gives None", func(t *testing.T) {
				wantDeep(t, none.ZipWith(monadz.Some("a"), sum).IsNone(), true)
				wantDeep(t, one.ZipWith(monadz.None[string](), sum).IsNone(), true)
			})
		})
	})
}

func TestOptionConversion(t *testing.T) {
	t.Run("Given Some(1) and None", func(t *testing.T) {
		some, none := monadz.Some(1), monadz.None[int]()
		t.Run("When OkOr is called", func(t *testing.T) {
			t.Run("Then Some gives Ok and None gives Err", func(t *testing.T) {
				v, err := some.OkOr(optErrTest).Get()
				wantDeep(t, v, 1)
				wantNil(t, err)
				wantDeep(t, none.OkOr(optErrTest).Err(), optErrTest)
			})
			t.Run("Then None with a nil error gives ErrNilError, not Ok", func(t *testing.T) {
				wantDeep(t, none.OkOr(nil).Err(), monadz.ErrNilError)
			})
		})
		t.Run("When OkOrElse is called", func(t *testing.T) {
			calls := 0
			f := func() error { calls++; return optErrTest }
			t.Run("Then Some gives Ok and does not call f", func(t *testing.T) {
				calls = 0
				wantDeep(t, some.OkOrElse(f).IsOk(), true)
				wantDeep(t, calls, 0)
			})
			t.Run("Then None gives Err(f())", func(t *testing.T) {
				calls = 0
				wantDeep(t, none.OkOrElse(f).Err(), optErrTest)
				wantDeep(t, calls, 1)
			})
		})
		t.Run("When All is ranged over", func(t *testing.T) {
			t.Run("Then Some yields one element", func(t *testing.T) {
				var got []int
				for v := range some.All() {
					got = append(got, v)
				}
				wantDeep(t, got, []int{1})
			})
			t.Run("Then None yields nothing", func(t *testing.T) {
				n := 0
				for range none.All() {
					n++
				}
				wantDeep(t, n, 0)
			})
			t.Run("Then a break after the first element is safe", func(t *testing.T) {
				n := 0
				for range some.All() {
					n++
					break
				}
				wantDeep(t, n, 1)
			})
		})
	})
}

func TestOptionInPlace(t *testing.T) {
	t.Run("Given an Option variable", func(t *testing.T) {
		t.Run("When Take is called on Some", func(t *testing.T) {
			o := monadz.Some(3)
			got := o.Take()
			t.Run("Then it returns the old value and leaves None", func(t *testing.T) {
				wantDeep(t, got, monadz.Some(3))
				wantDeep(t, o.IsNone(), true)
			})
		})
		t.Run("When Take is called on None", func(t *testing.T) {
			o := monadz.None[int]()
			got := o.Take()
			t.Run("Then both are None", func(t *testing.T) {
				wantDeep(t, got.IsNone(), true)
				wantDeep(t, o.IsNone(), true)
			})
		})
		t.Run("When TakeIf is called", func(t *testing.T) {
			t.Run("Then a passing value is taken", func(t *testing.T) {
				o := monadz.Some(4)
				wantDeep(t, o.TakeIf(optIsEven), monadz.Some(4))
				wantDeep(t, o.IsNone(), true)
			})
			t.Run("Then a failing value stays in place", func(t *testing.T) {
				o := monadz.Some(3)
				wantDeep(t, o.TakeIf(optIsEven).IsNone(), true)
				wantDeep(t, o, monadz.Some(3))
			})
			t.Run("Then None does not call the predicate", func(t *testing.T) {
				o := monadz.None[int]()
				called := false
				wantDeep(t, o.TakeIf(func(int) bool { called = true; return true }).IsNone(), true)
				wantDeep(t, called, false)
			})
		})
		t.Run("When Replace is called", func(t *testing.T) {
			t.Run("Then Some gives the old value and stores the new value", func(t *testing.T) {
				o := monadz.Some(1)
				wantDeep(t, o.Replace(2), monadz.Some(1))
				wantDeep(t, o, monadz.Some(2))
			})
			t.Run("Then None gives None and stores the new value", func(t *testing.T) {
				o := monadz.None[int]()
				wantDeep(t, o.Replace(2).IsNone(), true)
				wantDeep(t, o, monadz.Some(2))
			})
		})
		t.Run("When Insert is called", func(t *testing.T) {
			o := monadz.Some(1)
			p := o.Insert(5)
			t.Run("Then the Option holds the value and the pointer points into it", func(t *testing.T) {
				wantDeep(t, *p, 5)
				*p = 6
				wantDeep(t, o, monadz.Some(6))
			})
		})
		t.Run("When GetOrInsert is called", func(t *testing.T) {
			t.Run("Then None stores the value", func(t *testing.T) {
				o := monadz.None[int]()
				p := o.GetOrInsert(5)
				wantDeep(t, *p, 5)
				*p = 7
				wantDeep(t, o, monadz.Some(7))
			})
			t.Run("Then Some keeps its value", func(t *testing.T) {
				o := monadz.Some(1)
				wantDeep(t, *o.GetOrInsert(5), 1)
				wantDeep(t, o, monadz.Some(1))
			})
		})
		t.Run("When GetOrInsertWith is called", func(t *testing.T) {
			calls := 0
			f := func() int { calls++; return 5 }
			t.Run("Then None calls f and stores its value", func(t *testing.T) {
				calls = 0
				o := monadz.None[int]()
				p := o.GetOrInsertWith(f)
				wantDeep(t, *p, 5)
				wantDeep(t, calls, 1)
				*p = 8
				wantDeep(t, o, monadz.Some(8))
			})
			t.Run("Then Some does not call f", func(t *testing.T) {
				calls = 0
				o := monadz.Some(1)
				wantDeep(t, *o.GetOrInsertWith(f), 1)
				wantDeep(t, calls, 0)
			})
		})
	})
}
