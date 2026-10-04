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
				resEq(t, v.IsValid(), true)
				resEq(t, v.IsInvalid(), false)
				resEq(t, len(v.Errors()), 0)
				resNil(t, v.Err())
			})
		})
		t.Run("When Invalid is called with two errors and a nil", func(t *testing.T) {
			v := monadz.Invalid[int](resErrA, nil, resErrB)
			t.Run("Then it holds the two errors in order", func(t *testing.T) {
				resEq(t, v.IsInvalid(), true)
				resDeep(t, v.Errors(), []error{resErrA, resErrB})
			})
		})
		t.Run("When Invalid is called with only nil errors", func(t *testing.T) {
			v := monadz.Invalid[int](nil, nil)
			t.Run("Then it holds ErrNilError", func(t *testing.T) {
				resEq(t, v.IsInvalid(), true)
				resDeep(t, v.Errors(), []error{monadz.ErrNilError})
			})
		})
		t.Run("When Invalid is called with no errors", func(t *testing.T) {
			v := monadz.Invalid[int]()
			t.Run("Then it holds ErrNilError", func(t *testing.T) {
				resDeep(t, v.Errors(), []error{monadz.ErrNilError})
			})
		})
		t.Run("When Check is called", func(t *testing.T) {
			t.Run("Then true gives Valid and false gives Invalid(err)", func(t *testing.T) {
				resEq(t, monadz.Check(5, true, resErrA).IsValid(), true)
				resDeep(t, monadz.Check(5, false, resErrA).Errors(), []error{resErrA})
				resDeep(t, monadz.Check(5, false, nil).Errors(), []error{monadz.ErrNilError})
			})
		})
		t.Run("When the zero value is used", func(t *testing.T) {
			var v monadz.Validation[string]
			t.Run("Then it is Valid of the zero value", func(t *testing.T) {
				resEq(t, v.IsValid(), true)
				got, err := v.Get()
				resEq(t, got, "")
				resNil(t, err)
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
				resDeep(t, v.Errors(), []error{resErrA, resErrB})
			})
		})
		t.Run("When Err is called", func(t *testing.T) {
			err := v.Err()
			t.Run("Then it joins both errors", func(t *testing.T) {
				resEq(t, err.Error(), "a\nb")
				resEq(t, errors.Is(err, resErrA), true)
				resEq(t, errors.Is(err, resErrB), true)
			})
		})
		t.Run("When Get is called", func(t *testing.T) {
			got, err := v.Get()
			t.Run("Then it returns 0 and the joined error", func(t *testing.T) {
				resEq(t, got, 0)
				resEq(t, errors.Is(err, resErrB), true)
			})
		})
		t.Run("When Result is called", func(t *testing.T) {
			t.Run("Then Invalid gives Err with the joined errors and Valid gives Ok", func(t *testing.T) {
				r := v.Result()
				resEq(t, errors.Is(r.Err(), resErrA), true)
				resEq(t, errors.Is(r.Err(), resErrB), true)
				resEq(t, monadz.Valid(3).Result().Must(), 3)
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
				resEq(t, got, "4")
				resDeep(t, bad.Map(f).Errors(), []error{resErrA, resErrB})
				resEq(t, calls, 1)
			})
		})
		t.Run("When AndThen is called", func(t *testing.T) {
			calls := 0
			f := func(n int) monadz.Validation[int] {
				calls++
				return monadz.Check(n, n > 10, errors.New("too small"))
			}
			t.Run("Then it is fail-fast: f runs only for Valid", func(t *testing.T) {
				resEq(t, ok.AndThen(f).Errors()[0].Error(), "too small")
				resDeep(t, bad.AndThen(f).Errors(), []error{resErrA, resErrB})
				resEq(t, calls, 1)
			})
		})
		t.Run("When Ensure is called", func(t *testing.T) {
			even := func(n int) bool { return n%2 == 0 }
			t.Run("Then a pass keeps the value, a fail gives the error, and Invalid does not change", func(t *testing.T) {
				resEq(t, ok.Ensure(even, resErrA).IsValid(), true)
				resDeep(t, monadz.Valid(3).Ensure(even, resErrA).Errors(), []error{resErrA})
				resDeep(t, bad.Ensure(func(int) bool { panic("not called") }, errors.New("c")).Errors(), []error{resErrA, resErrB})
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
				resEq(t, len(errs), 2)
				for i, want := range []error{resErrA, resErrB} {
					var fe *monadz.FieldError
					resEq(t, errors.As(errs[i], &fe), true)
					resEq(t, fe.Field, "email")
					resEq(t, fe.Unwrap(), want)
					resEq(t, errors.Is(errs[i], want), true)
				}
				resEq(t, errs[0].Error(), "email: a")
				resEq(t, errors.Is(v.Err(), resErrB), true)
			})
		})
		t.Run("When Field is called on a Valid value", func(t *testing.T) {
			got, err := monadz.Valid("x").Field("email").Get()
			t.Run("Then the value does not change", func(t *testing.T) {
				resEq(t, got, "x")
				resNil(t, err)
			})
		})
	})
}

func TestValidationString(t *testing.T) {
	t.Run("Given Valid and Invalid Validations", func(t *testing.T) {
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then it gives Valid(<v>) or Invalid(<err>; <err>)", func(t *testing.T) {
				resEq(t, monadz.Valid(1).String(), "Valid(1)")
				resEq(t, monadz.Invalid[int](resErrA, resErrB).String(), "Invalid(a; b)")
				resEq(t, monadz.Invalid[int](resErrA).Field("f").String(), "Invalid(f: a)")
			})
		})
	})
}
