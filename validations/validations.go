// Package validations combines monadz.Validation values and collects every
// error, instead of stopping at the first one. (Haskell: the Applicative
// instance of Data.Validation.)
//
//	form := validations.Map3(
//		email(req.Email).Field("email"),
//		password(req.Password).Field("password"),
//		company(req.Company).Field("company"),
//		newSignup,
//	)
//	// form is Valid(signup), or Invalid with ALL field errors.
//
// To combine only two values, use the method monadz.Validation.ZipWith. To
// make one check, use monadz.ValidIf.
package validations

import (
	"iter"
	"slices"

	"github.com/zkrebbekx/monadz"
)

// Map2 combines 2 Validations with combine. If any is Invalid, Map2 returns
// Invalid with the errors of ALL of them, in argument order.
func Map2[A, B, R any](a monadz.Validation[A], b monadz.Validation[B], combine func(A, B) R) monadz.Validation[R] {
	if errs := slices.Concat(a.Errors(), b.Errors()); len(errs) > 0 {
		return monadz.Invalid[R](errs...)
	}
	va, _ := a.Get()
	vb, _ := b.Get()
	return monadz.Valid(combine(va, vb))
}

// Map3 combines 3 Validations with combine. If any is Invalid, Map3 returns
// Invalid with the errors of ALL of them, in argument order.
func Map3[A, B, C, R any](a monadz.Validation[A], b monadz.Validation[B], c monadz.Validation[C], combine func(A, B, C) R) monadz.Validation[R] {
	if errs := slices.Concat(a.Errors(), b.Errors(), c.Errors()); len(errs) > 0 {
		return monadz.Invalid[R](errs...)
	}
	va, _ := a.Get()
	vb, _ := b.Get()
	vc, _ := c.Get()
	return monadz.Valid(combine(va, vb, vc))
}

// Map4 combines 4 Validations with combine. If any is Invalid, Map4 returns
// Invalid with the errors of ALL of them, in argument order.
func Map4[A, B, C, D, R any](a monadz.Validation[A], b monadz.Validation[B], c monadz.Validation[C], d monadz.Validation[D], combine func(A, B, C, D) R) monadz.Validation[R] {
	if errs := slices.Concat(a.Errors(), b.Errors(), c.Errors(), d.Errors()); len(errs) > 0 {
		return monadz.Invalid[R](errs...)
	}
	va, _ := a.Get()
	vb, _ := b.Get()
	vc, _ := c.Get()
	vd, _ := d.Get()
	return monadz.Valid(combine(va, vb, vc, vd))
}

// Map5 combines 5 Validations with combine. If any is Invalid, Map5 returns
// Invalid with the errors of ALL of them, in argument order.
func Map5[A, B, C, D, E, R any](a monadz.Validation[A], b monadz.Validation[B], c monadz.Validation[C], d monadz.Validation[D], e monadz.Validation[E], combine func(A, B, C, D, E) R) monadz.Validation[R] {
	if errs := slices.Concat(a.Errors(), b.Errors(), c.Errors(), d.Errors(), e.Errors()); len(errs) > 0 {
		return monadz.Invalid[R](errs...)
	}
	va, _ := a.Get()
	vb, _ := b.Get()
	vc, _ := c.Get()
	vd, _ := d.Get()
	ve, _ := e.Get()
	return monadz.Valid(combine(va, vb, vc, vd, ve))
}

// Map6 combines 6 Validations with combine. If any is Invalid, Map6 returns
// Invalid with the errors of ALL of them, in argument order.
func Map6[A, B, C, D, E, F, R any](a monadz.Validation[A], b monadz.Validation[B], c monadz.Validation[C], d monadz.Validation[D], e monadz.Validation[E], f monadz.Validation[F], combine func(A, B, C, D, E, F) R) monadz.Validation[R] {
	if errs := slices.Concat(a.Errors(), b.Errors(), c.Errors(), d.Errors(), e.Errors(), f.Errors()); len(errs) > 0 {
		return monadz.Invalid[R](errs...)
	}
	va, _ := a.Get()
	vb, _ := b.Get()
	vc, _ := c.Get()
	vd, _ := d.Get()
	ve, _ := e.Get()
	vf, _ := f.Get()
	return monadz.Valid(combine(va, vb, vc, vd, ve, vf))
}

// Collect returns Valid of all values if every Validation is Valid, else
// Invalid with every error. It reads the whole sequence.
func Collect[T any](seq iter.Seq[monadz.Validation[T]]) monadz.Validation[[]T] {
	out := []T{}
	var errs []error
	for v := range seq {
		if v.IsInvalid() {
			errs = append(errs, v.Errors()...)
			continue
		}
		val, _ := v.Get()
		out = append(out, val)
	}
	if len(errs) > 0 {
		return monadz.Invalid[[]T](errs...)
	}
	return monadz.Valid(out)
}

// Traverse applies f to each item and collects the results and every error.
// Use it to validate each row of an import file.
func Traverse[A, B any](seq iter.Seq[A], f func(A) monadz.Validation[B]) monadz.Validation[[]B] {
	return Collect(func(yield func(monadz.Validation[B]) bool) {
		for a := range seq {
			if !yield(f(a)) {
				return
			}
		}
	})
}

// Errors runs independent checks and returns every error, or nil. Use it
// when the checks produce no value.
func Errors(vs ...monadz.Validation[monadz.Unit]) []error {
	var errs []error
	for _, v := range vs {
		errs = append(errs, v.Errors()...)
	}
	return errs
}
