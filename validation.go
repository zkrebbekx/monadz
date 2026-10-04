package monadz

import (
	"errors"
	"fmt"
	"strings"
)

// Validation holds a value of type T (Valid) or one or more errors
// (Invalid).
//
// Result stops at the first error. Validation collects ALL the errors, so
// a form can report every bad field in one response. Combine Validations
// with the Map2 ... Map6 functions in package validations, which run every
// check and join the errors. (Haskell: Data.Validation.)
//
// AndThen is fail-fast, because the next check needs the value. Use the
// validations package for independent checks.
//
// The zero value is Valid(zero T).
type Validation[T any] struct {
	value T
	errs  []error
}

// Valid returns a Validation that holds v.
func Valid[T any](v T) Validation[T] { return Validation[T]{value: v} }

// Invalid returns a Validation that holds errs. Nil errors are dropped. If
// no error is left, the Validation holds ErrNilError.
func Invalid[T any](errs ...error) Validation[T] {
	kept := make([]error, 0, len(errs))
	for _, e := range errs {
		if e != nil {
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 {
		kept = append(kept, ErrNilError)
	}
	return Validation[T]{errs: kept}
}

// Check returns Valid(v) if ok is true, else Invalid(err).
func Check[T any](v T, ok bool, err error) Validation[T] {
	if !ok {
		return Invalid[T](err)
	}
	return Valid(v)
}

// IsValid reports whether the Validation holds a value.
func (v Validation[T]) IsValid() bool { return len(v.errs) == 0 }

// IsInvalid reports whether the Validation holds errors.
func (v Validation[T]) IsInvalid() bool { return len(v.errs) > 0 }

// Errors returns a copy of the errors. It is empty for a Valid value.
func (v Validation[T]) Errors() []error { return append([]error(nil), v.errs...) }

// Err returns the errors joined with errors.Join, or nil.
func (v Validation[T]) Err() error { return errors.Join(v.errs...) }

// Get returns the value and nil, or the zero value and the joined errors.
func (v Validation[T]) Get() (T, error) {
	if len(v.errs) > 0 {
		var zero T
		return zero, v.Err()
	}
	return v.value, nil
}

// Result converts the Validation to a Result with the joined errors.
func (v Validation[T]) Result() Result[T] { return Of(v.Get()) }

// Map applies f to the value.
func (v Validation[T]) Map[U any](f func(T) U) Validation[U] {
	if len(v.errs) > 0 {
		return Validation[U]{errs: v.errs}
	}
	return Valid(f(v.value))
}

// AndThen applies f to the value and returns its Validation. It is
// fail-fast: f does not run for an Invalid value.
func (v Validation[T]) AndThen[U any](f func(T) Validation[U]) Validation[U] {
	if len(v.errs) > 0 {
		return Validation[U]{errs: v.errs}
	}
	return f(v.value)
}

// Ensure returns Invalid(err) if the Validation is Valid and its value does
// not satisfy pred. An Invalid Validation is returned unchanged, and pred
// does not run.
func (v Validation[T]) Ensure(pred func(T) bool, err error) Validation[T] {
	if len(v.errs) == 0 && !pred(v.value) {
		return Invalid[T](err)
	}
	return v
}

// Field wraps every error in a *FieldError with the given field name, so an
// API can report which field failed.
func (v Validation[T]) Field(name string) Validation[T] {
	if len(v.errs) == 0 {
		return v
	}
	wrapped := make([]error, len(v.errs))
	for i, e := range v.errs {
		wrapped[i] = &FieldError{Field: name, Err: e}
	}
	return Validation[T]{errs: wrapped}
}

// String returns "Valid(<value>)" or "Invalid(<err>; <err>)".
func (v Validation[T]) String() string {
	if len(v.errs) == 0 {
		return fmt.Sprintf("Valid(%v)", v.value)
	}
	msgs := make([]string, len(v.errs))
	for i, e := range v.errs {
		msgs[i] = e.Error()
	}
	return "Invalid(" + strings.Join(msgs, "; ") + ")"
}

// FieldError is an error about one input field.
type FieldError struct {
	Field string
	Err   error
}

// Error returns "<field>: <err>".
func (e *FieldError) Error() string { return e.Field + ": " + e.Err.Error() }

// Unwrap returns the wrapped error.
func (e *FieldError) Unwrap() error { return e.Err }
