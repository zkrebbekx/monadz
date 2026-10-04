package validations_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/zkrebbekx/monadz"
	"github.com/zkrebbekx/monadz/validations"
)

func resEq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func resDeep(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// resErrs returns n distinct errors e0 ... e(n-1).
func resErrs(n int) []error {
	errs := make([]error, n)
	for i := range errs {
		errs[i] = fmt.Errorf("e%d", i)
	}
	return errs
}

// resV returns Valid(i), or Invalid(err) when err is not nil.
func resV(i int, err error) monadz.Validation[int] {
	if err != nil {
		return monadz.Invalid[int](err)
	}
	return monadz.Valid(i)
}

func TestMapN(t *testing.T) {
	t.Run("Given the Map2 ... Map6 functions", func(t *testing.T) {
		e := resErrs(6)
		// call runs MapN with the arity n. errs[i] makes argument i Invalid.
		call := func(n int, errs []error, called *bool) monadz.Validation[int] {
			v := func(i int) monadz.Validation[int] { return resV(i+1, errs[i]) }
			switch n {
			case 2:
				return validations.Map2(v(0), v(1), func(a, b int) int { *called = true; return a + b })
			case 3:
				return validations.Map3(v(0), v(1), v(2), func(a, b, c int) int { *called = true; return a + b + c })
			case 4:
				return validations.Map4(v(0), v(1), v(2), v(3), func(a, b, c, d int) int { *called = true; return a + b + c + d })
			case 5:
				return validations.Map5(v(0), v(1), v(2), v(3), v(4), func(a, b, c, d, e int) int { *called = true; return a + b + c + d + e })
			default:
				return validations.Map6(v(0), v(1), v(2), v(3), v(4), v(5), func(a, b, c, d, e, f int) int { *called = true; return a + b + c + d + e + f })
			}
		}
		for n := 2; n <= 6; n++ {
			t.Run(fmt.Sprintf("When Map%d gets only Valid values", n), func(t *testing.T) {
				called := false
				got := call(n, make([]error, 6), &called)
				t.Run("Then combine runs once with all values", func(t *testing.T) {
					v, err := got.Get()
					resEq(t, err, nil)
					resEq(t, v, n*(n+1)/2)
					resEq(t, called, true)
				})
			})
			t.Run(fmt.Sprintf("When Map%d gets Invalid first and last arguments", n), func(t *testing.T) {
				errs := make([]error, 6)
				errs[0], errs[n-1] = e[0], e[n-1]
				called := false
				got := call(n, errs, &called)
				t.Run("Then it collects both errors in argument order and does not call combine", func(t *testing.T) {
					resDeep(t, got.Errors(), []error{e[0], e[n-1]})
					resEq(t, called, false)
				})
			})
			t.Run(fmt.Sprintf("When Map%d gets only Invalid arguments", n), func(t *testing.T) {
				called := false
				got := call(n, e, &called)
				t.Run("Then it collects all errors in argument order", func(t *testing.T) {
					resDeep(t, got.Errors(), e[:n])
					resEq(t, called, false)
				})
			})
		}
		t.Run("When an argument holds two errors", func(t *testing.T) {
			got := validations.Map2(monadz.Invalid[int](e[0], e[1]), monadz.Invalid[string](e[2]), func(int, string) bool { return true })
			t.Run("Then every error of every argument is kept", func(t *testing.T) {
				resDeep(t, got.Errors(), []error{e[0], e[1], e[2]})
			})
		})
	})
}

func TestCollectTraverse(t *testing.T) {
	t.Run("Given a sequence of Validations", func(t *testing.T) {
		e := resErrs(3)
		t.Run("When every Validation is Valid", func(t *testing.T) {
			got := validations.Collect(slices.Values([]monadz.Validation[int]{monadz.Valid(1), monadz.Valid(2)}))
			t.Run("Then Collect returns Valid of all values", func(t *testing.T) {
				v, err := got.Get()
				resEq(t, err, nil)
				resDeep(t, v, []int{1, 2})
			})
		})
		t.Run("When the sequence is empty", func(t *testing.T) {
			got := validations.Collect(slices.Values([]monadz.Validation[int]{}))
			t.Run("Then Collect returns Valid of an empty, non-nil slice", func(t *testing.T) {
				v, _ := got.Get()
				resDeep(t, v, []int{})
			})
		})
		t.Run("When some Validations are Invalid", func(t *testing.T) {
			read := 0
			seq := func(yield func(monadz.Validation[int]) bool) {
				for _, v := range []monadz.Validation[int]{monadz.Invalid[int](e[0]), monadz.Valid(1), monadz.Invalid[int](e[1], e[2])} {
					read++
					if !yield(v) {
						return
					}
				}
			}
			got := validations.Collect(seq)
			t.Run("Then Collect reads the whole sequence and returns every error in order", func(t *testing.T) {
				resDeep(t, got.Errors(), e)
				resEq(t, read, 3)
			})
		})
		t.Run("When Traverse checks each item", func(t *testing.T) {
			calls := 0
			positive := func(n int) monadz.Validation[int] {
				calls++
				return monadz.Check(n*10, n > 0, fmt.Errorf("bad %d", n))
			}
			ok := validations.Traverse(slices.Values([]int{1, 2}), positive)
			bad := validations.Traverse(slices.Values([]int{-1, 2, -3}), positive)
			t.Run("Then it maps every item and collects every error", func(t *testing.T) {
				v, _ := ok.Get()
				resDeep(t, v, []int{10, 20})
				resEq(t, bad.String(), "Invalid(bad -1; bad -3)")
				resEq(t, calls, 5)
			})
		})
	})
}

func TestErrors(t *testing.T) {
	t.Run("Given independent checks with no value", func(t *testing.T) {
		e := resErrs(2)
		t.Run("When every check passes", func(t *testing.T) {
			got := validations.Errors(monadz.Valid(monadz.Unit{}), monadz.Valid(monadz.Unit{}))
			t.Run("Then Errors returns nil", func(t *testing.T) {
				resEq(t, got == nil, true)
			})
		})
		t.Run("When two checks fail", func(t *testing.T) {
			got := validations.Errors(monadz.Invalid[monadz.Unit](e[0]), monadz.Valid(monadz.Unit{}), monadz.Invalid[monadz.Unit](e[1]))
			t.Run("Then Errors returns both errors in order", func(t *testing.T) {
				resDeep(t, got, e)
				resEq(t, errors.Is(errors.Join(got...), e[1]), true)
			})
		})
	})
}
