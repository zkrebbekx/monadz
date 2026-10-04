package monadz_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/zkrebbekx/monadz"
)

var (
	resErrA = errors.New("a")
	resErrB = errors.New("b")
)

func TestValidationConstructors(t *testing.T) {
	t.Run("Given the Validation constructors", func(t *testing.T) {
		t.Run("When Valid is called with 1", func(t *testing.T) {
			v := monadz.Valid(1)
			t.Run("Then the Validation is valid with no errors", func(t *testing.T) {
				wantEq(t, v.IsValid(), true)
				wantEq(t, v.IsInvalid(), false)
				wantEq(t, len(v.Errors()), 0)
				wantNil(t, v.Err())
			})
		})
		t.Run("When Invalid is called with two errors and a nil", func(t *testing.T) {
			v := monadz.Invalid[int](resErrA, nil, resErrB)
			t.Run("Then it holds the two errors in order", func(t *testing.T) {
				wantEq(t, v.IsInvalid(), true)
				wantDeep(t, v.Errors(), []error{resErrA, resErrB})
			})
		})
		t.Run("When Invalid is called with only nil errors", func(t *testing.T) {
			v := monadz.Invalid[int](nil, nil)
			t.Run("Then it holds ErrNilError", func(t *testing.T) {
				wantEq(t, v.IsInvalid(), true)
				wantDeep(t, v.Errors(), []error{monadz.ErrNilError})
			})
		})
		t.Run("When Invalid is called with no errors", func(t *testing.T) {
			v := monadz.Invalid[int]()
			t.Run("Then it holds ErrNilError", func(t *testing.T) {
				wantDeep(t, v.Errors(), []error{monadz.ErrNilError})
			})
		})
		t.Run("When ValidIf is called", func(t *testing.T) {
			t.Run("Then true gives Valid and false gives Invalid(err)", func(t *testing.T) {
				wantEq(t, monadz.ValidIf(5, true, resErrA).IsValid(), true)
				wantDeep(t, monadz.ValidIf(5, false, resErrA).Errors(), []error{resErrA})
				wantDeep(t, monadz.ValidIf(5, false, nil).Errors(), []error{monadz.ErrNilError})
			})
		})
		t.Run("When the zero value is used", func(t *testing.T) {
			var v monadz.Validation[string]
			t.Run("Then it is Valid of the zero value", func(t *testing.T) {
				wantEq(t, v.IsValid(), true)
				got, err := v.Get()
				wantEq(t, got, "")
				wantNil(t, err)
			})
		})
	})
}

func TestValidationAccessors(t *testing.T) {
	t.Run("Given an Invalid Validation with two errors", func(t *testing.T) {
		v := monadz.Invalid[int](resErrA, resErrB)
		t.Run("When the slice from Errors is changed", func(t *testing.T) {
			errs := v.Errors()
			errs[0] = errors.New("changed")
			t.Run("Then the Validation does not change", func(t *testing.T) {
				wantDeep(t, v.Errors(), []error{resErrA, resErrB})
			})
		})
		t.Run("When Err is called", func(t *testing.T) {
			err := v.Err()
			t.Run("Then it joins both errors", func(t *testing.T) {
				wantEq(t, err.Error(), "a\nb")
				wantEq(t, errors.Is(err, resErrA), true)
				wantEq(t, errors.Is(err, resErrB), true)
			})
		})
		t.Run("When Get is called", func(t *testing.T) {
			got, err := v.Get()
			t.Run("Then it returns 0 and the joined error", func(t *testing.T) {
				wantEq(t, got, 0)
				wantEq(t, errors.Is(err, resErrB), true)
			})
		})
		t.Run("When Result is called", func(t *testing.T) {
			t.Run("Then Invalid gives Err with the joined errors and Valid gives Ok", func(t *testing.T) {
				r := v.Result()
				wantEq(t, errors.Is(r.Err(), resErrA), true)
				wantEq(t, errors.Is(r.Err(), resErrB), true)
				wantEq(t, monadz.Valid(3).Result().Must(), 3)
			})
		})
	})
}

func TestValidationTransformation(t *testing.T) {
	t.Run("Given a Valid(4) and an Invalid(a, b)", func(t *testing.T) {
		ok := monadz.Valid(4)
		bad := monadz.Invalid[int](resErrA, resErrB)
		t.Run("When Map is called", func(t *testing.T) {
			calls := 0
			f := func(n int) string { calls++; return strconv.Itoa(n) }
			t.Run("Then Valid maps the value and Invalid keeps its errors without a call", func(t *testing.T) {
				got, _ := ok.Map(f).Get()
				wantEq(t, got, "4")
				wantDeep(t, bad.Map(f).Errors(), []error{resErrA, resErrB})
				wantEq(t, calls, 1)
			})
		})
		t.Run("When AndThen is called", func(t *testing.T) {
			calls := 0
			f := func(n int) monadz.Validation[int] {
				calls++
				return monadz.ValidIf(n, n > 10, errors.New("too small"))
			}
			t.Run("Then it is fail-fast: f runs only for Valid", func(t *testing.T) {
				wantEq(t, ok.AndThen(f).Errors()[0].Error(), "too small")
				wantDeep(t, bad.AndThen(f).Errors(), []error{resErrA, resErrB})
				wantEq(t, calls, 1)
			})
		})
		t.Run("When Ensure is called", func(t *testing.T) {
			even := func(n int) bool { return n%2 == 0 }
			t.Run("Then a pass keeps the value, a fail gives the error, and Invalid does not change", func(t *testing.T) {
				wantEq(t, ok.Ensure(even, resErrA).IsValid(), true)
				wantDeep(t, monadz.Valid(3).Ensure(even, resErrA).Errors(), []error{resErrA})
				wantDeep(t, bad.Ensure(func(int) bool { panic("not called") }, errors.New("c")).Errors(), []error{resErrA, resErrB})
			})
		})
	})
}

func TestValidationField(t *testing.T) {
	t.Run("Given an Invalid Validation with two errors", func(t *testing.T) {
		v := monadz.Invalid[string](resErrA, resErrB).Field("email")
		t.Run("When Field wraps the errors", func(t *testing.T) {
			errs := v.Errors()
			t.Run("Then each error is a *FieldError with the field name and the cause", func(t *testing.T) {
				wantEq(t, len(errs), 2)
				for i, want := range []error{resErrA, resErrB} {
					var fe *monadz.FieldError
					wantEq(t, errors.As(errs[i], &fe), true)
					wantEq(t, fe.Field, "email")
					wantEq(t, fe.Unwrap(), want)
					wantEq(t, errors.Is(errs[i], want), true)
				}
				wantEq(t, errs[0].Error(), "email: a")
				wantEq(t, errors.Is(v.Err(), resErrB), true)
			})
		})
		t.Run("When Field is called on a Valid value", func(t *testing.T) {
			got, err := monadz.Valid("x").Field("email").Get()
			t.Run("Then the value does not change", func(t *testing.T) {
				wantEq(t, got, "x")
				wantNil(t, err)
			})
		})
	})
}

func TestValidationString(t *testing.T) {
	t.Run("Given Valid and Invalid Validations", func(t *testing.T) {
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then it gives Valid(<v>) or Invalid(<err>; <err>)", func(t *testing.T) {
				wantEq(t, monadz.Valid(1).String(), "Valid(1)")
				wantEq(t, monadz.Invalid[int](resErrA, resErrB).String(), "Invalid(a; b)")
				wantEq(t, monadz.Invalid[int](resErrA).Field("f").String(), "Invalid(f: a)")
			})
		})
	})
}

func TestValidationSingleError(t *testing.T) {
	t.Run("Given an Invalid Validation with exactly one error", func(t *testing.T) {
		v := monadz.Invalid[int](resErrA)
		t.Run("When Err and Get are called", func(t *testing.T) {
			_, err := v.Get()
			t.Run("Then both return that error itself, not a joined error", func(t *testing.T) {
				wantEq(t, v.Err(), resErrA)
				wantEq(t, err, resErrA)
			})
		})
	})
	t.Run("Given a failed Result", func(t *testing.T) {
		r := monadz.Err[int](resErrA)
		t.Run("When it goes to a Validation and back to a Result", func(t *testing.T) {
			got := r.Validation().Result().Err()
			t.Run("Then the error keeps its identity", func(t *testing.T) {
				wantEq(t, got, resErrA)
			})
		})
	})
}

func TestFieldErrorNilErr(t *testing.T) {
	t.Run("Given a FieldError with a nil Err", func(t *testing.T) {
		e := &monadz.FieldError{Field: "email"}
		t.Run("When Error is called", func(t *testing.T) {
			var msg string
			wantNoPanic(t, func() { msg = e.Error() })
			t.Run("Then it returns the field name and <nil>", func(t *testing.T) {
				wantEq(t, msg, "email: <nil>")
			})
		})
	})
}

func TestValidationExtraction(t *testing.T) {
	t.Run("Given Valid(5) and an Invalid Validation with two errors", func(t *testing.T) {
		ok := monadz.Valid(5)
		bad := monadz.Invalid[int](resErrA, resErrB)
		t.Run("When Match is called", func(t *testing.T) {
			onValid := func(n int) string { return "valid " + strconv.Itoa(n) }
			onInvalid := func(errs []error) string { return "invalid " + strconv.Itoa(len(errs)) }
			t.Run("Then Valid calls onValid and Invalid calls onInvalid with every error", func(t *testing.T) {
				wantEq(t, ok.Match(onValid, onInvalid), "valid 5")
				wantEq(t, bad.Match(onValid, onInvalid), "invalid 2")
			})
		})
		t.Run("When ValueOr, ValueOrElse, and ValueOrZero are called", func(t *testing.T) {
			calls := 0
			count := func(errs []error) int { calls++; return len(errs) * 10 }
			t.Run("Then Valid gives the value and Invalid gives the fallback", func(t *testing.T) {
				wantEq(t, ok.ValueOr(9), 5)
				wantEq(t, bad.ValueOr(9), 9)
				wantEq(t, ok.ValueOrElse(count), 5)
				wantEq(t, calls, 0)
				wantEq(t, bad.ValueOrElse(count), 20)
				wantEq(t, calls, 1)
				wantEq(t, ok.ValueOrZero(), 5)
				wantEq(t, bad.ValueOrZero(), 0)
			})
		})
		t.Run("When Must is called", func(t *testing.T) {
			t.Run("Then Valid gives the value and Invalid panics with Err()", func(t *testing.T) {
				wantEq(t, ok.Must(), 5)
				p := mustPanic(t, func() { bad.Must() })
				err, isErr := p.(error)
				wantTrue(t, isErr, "the panic value is an error")
				if isErr {
					wantEq(t, err.Error(), "a\nb")
				}
				wantPanic(t, func() { monadz.Invalid[int](resErrA).Must() }, resErrA)
			})
		})
		t.Run("When Expect is called", func(t *testing.T) {
			t.Run("Then Valid gives the value and Invalid panics with msg and Err()", func(t *testing.T) {
				wantEq(t, ok.Expect("form"), 5)
				p := mustPanic(t, func() { bad.Expect("form") })
				err, isErr := p.(error)
				wantTrue(t, isErr, "the panic value is an error")
				if isErr {
					wantEq(t, err.Error(), "form: a\nb")
					wantTrue(t, errors.Is(err, resErrB), "errors.Is(err, b)")
				}
			})
		})
		t.Run("When Inspect is called", func(t *testing.T) {
			var seen []int
			gotOk := ok.Inspect(func(n int) { seen = append(seen, n) })
			gotBad := bad.Inspect(func(n int) { seen = append(seen, n) })
			t.Run("Then f sees only the Valid value, and both Validations are unchanged", func(t *testing.T) {
				wantDeep(t, seen, []int{5})
				wantDeep(t, gotOk, ok)
				wantDeep(t, gotBad, bad)
			})
		})
	})
}

func TestValidationMapErr(t *testing.T) {
	t.Run("Given an Invalid Validation with two errors", func(t *testing.T) {
		bad := monadz.Invalid[int](resErrA, resErrB)
		t.Run("When MapErr wraps each error", func(t *testing.T) {
			got := bad.MapErr(func(e error) error { return &monadz.FieldError{Field: "x", Err: e} })
			t.Run("Then f applies to each error, in order", func(t *testing.T) {
				wantEq(t, got.Err().Error(), "x: a\nx: b")
				wantTrue(t, errors.Is(got.Err(), resErrB), "errors.Is(err, b)")
			})
		})
		t.Run("When f returns nil for an error", func(t *testing.T) {
			got := bad.MapErr(func(e error) error {
				if e == resErrA {
					return nil
				}
				return e
			})
			t.Run("Then that error becomes ErrNilError", func(t *testing.T) {
				wantDeep(t, got.Errors(), []error{monadz.ErrNilError, resErrB})
			})
		})
	})
	t.Run("Given Valid(5)", func(t *testing.T) {
		calls := 0
		got := monadz.Valid(5).MapErr(func(e error) error { calls++; return e })
		t.Run("When MapErr is called", func(t *testing.T) {
			t.Run("Then f does not run and the value stays", func(t *testing.T) {
				wantEq(t, calls, 0)
				wantDeep(t, got, monadz.Valid(5))
			})
		})
	})
}

func TestValidationZipWith(t *testing.T) {
	add := func(a int, b string) string { return strconv.Itoa(a) + b }
	t.Run("Given two Valid Validations", func(t *testing.T) {
		t.Run("When ZipWith combines them", func(t *testing.T) {
			got := monadz.Valid(1).ZipWith(monadz.Valid("x"), add)
			t.Run("Then it returns Valid of f(a, b)", func(t *testing.T) {
				wantDeep(t, got, monadz.Valid("1x"))
			})
		})
	})
	t.Run("Given two Invalid Validations", func(t *testing.T) {
		t.Run("When ZipWith combines them", func(t *testing.T) {
			got := monadz.Invalid[int](resErrA).ZipWith(monadz.Invalid[string](resErrB), add)
			t.Run("Then it collects the errors of both, in order", func(t *testing.T) {
				wantDeep(t, got.Errors(), []error{resErrA, resErrB})
			})
		})
	})
	t.Run("Given one Valid and one Invalid Validation", func(t *testing.T) {
		t.Run("When ZipWith combines them in each order", func(t *testing.T) {
			left := monadz.Invalid[int](resErrA).ZipWith(monadz.Valid("x"), add)
			right := monadz.Valid(1).ZipWith(monadz.Invalid[string](resErrB), add)
			t.Run("Then each result holds only the error of the Invalid one", func(t *testing.T) {
				wantDeep(t, left.Errors(), []error{resErrA})
				wantDeep(t, right.Errors(), []error{resErrB})
			})
		})
	})
}
