package monadz

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
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
	value T // invariant: value is the zero value of T when errs is not empty
	errs  []error
}

// Valid returns a Validation that holds v.
func Valid[T any](v T) Validation[T] { return Validation[T]{value: v} }

// Invalid returns a Validation that holds errs. Invalid drops nil errors. If
// no error is left, the Validation holds ErrNilError.
func Invalid[T any](errs ...error) Validation[T] {
	kept := slices.DeleteFunc(slices.Clone(errs), func(e error) bool { return e == nil })
	if len(kept) == 0 {
		kept = []error{ErrNilError}
	}
	return Validation[T]{errs: kept}
}

// ValidIf returns Valid(v) if ok is true, else Invalid(err).
func ValidIf[T any](v T, ok bool, err error) Validation[T] {
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

// Err returns nil for a Valid value. If the Validation holds exactly one
// error, Err returns that error itself. Else Err returns the errors joined
// with errors.Join.
func (v Validation[T]) Err() error {
	if len(v.errs) == 1 {
		return v.errs[0]
	}
	return errors.Join(v.errs...)
}

// Get returns the value and nil, or the zero value and Err().
func (v Validation[T]) Get() (T, error) { return v.value, v.Err() }

// ---- Extraction ----

// Must returns the value, or panics with Err(). (Rust: unwrap.)
func (v Validation[T]) Must() T {
	if len(v.errs) > 0 {
		panic(v.Err())
	}
	return v.value
}

// Expect returns the value, or panics with msg and Err(). (Rust: expect.)
func (v Validation[T]) Expect(msg string) T {
	if len(v.errs) > 0 {
		panic(fmt.Errorf("%s: %w", msg, v.Err()))
	}
	return v.value
}

// ValueOr returns the value, or fallback for an Invalid value.
// (Rust: unwrap_or.)
func (v Validation[T]) ValueOr(fallback T) T {
	if len(v.errs) > 0 {
		return fallback
	}
	return v.value
}

// ValueOrElse returns the value, or f applied to the errors. f runs only for
// an Invalid value. (Rust: unwrap_or_else.)
func (v Validation[T]) ValueOrElse(f func([]error) T) T {
	if len(v.errs) > 0 {
		return f(v.Errors())
	}
	return v.value
}

// ValueOrZero returns the value, or the zero value of T.
// (Rust: unwrap_or_default.)
func (v Validation[T]) ValueOrZero() T { return v.value }

// Match calls onValid with the value or onInvalid with the errors, and
// returns the result. (Haskell: validation, in package Data.Validation.)
func (v Validation[T]) Match[U any](onValid func(T) U, onInvalid func([]error) U) U {
	if len(v.errs) > 0 {
		return onInvalid(v.Errors())
	}
	return onValid(v.value)
}

// ---- Transformation ----

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
// not satisfy pred. Ensure returns an Invalid Validation unchanged, and pred
// does not run.
func (v Validation[T]) Ensure(pred func(T) bool, err error) Validation[T] {
	if len(v.errs) == 0 && !pred(v.value) {
		return Invalid[T](err)
	}
	return v
}

// MapErr applies f to each error. If f returns nil, MapErr stores
// ErrNilError. f does not run for a Valid value.
func (v Validation[T]) MapErr(f func(error) error) Validation[T] {
	if len(v.errs) == 0 {
		return v
	}
	mapped := make([]error, len(v.errs))
	for i, e := range v.errs {
		mapped[i] = cmp.Or(f(e), ErrNilError)
	}
	return Validation[T]{errs: mapped}
}

// Inspect calls f with the value, if any, and returns the Validation
// unchanged.
func (v Validation[T]) Inspect(f func(T)) Validation[T] {
	if len(v.errs) == 0 {
		f(v.value)
	}
	return v
}

// ZipWith combines both values with f if both are Valid. Else ZipWith
// returns Invalid with the errors of both, in order. Unlike AndThen, it
// collects every error. (Haskell: liftA2 for Data.Validation.)
func (v Validation[T]) ZipWith[U, V any](other Validation[U], f func(T, U) V) Validation[V] {
	if errs := slices.Concat(v.errs, other.errs); len(errs) > 0 {
		return Validation[V]{errs: errs}
	}
	return Valid(f(v.value, other.value))
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
		return fmt.Sprintf("Valid(%v)", printable(v.value))
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

// Error returns "<field>: <err>", or "<field>: <nil>" for a nil Err.
func (e *FieldError) Error() string {
	if e.Err == nil {
		return e.Field + ": <nil>"
	}
	return e.Field + ": " + e.Err.Error()
}

// Unwrap returns the wrapped error.
func (e *FieldError) Unwrap() error { return e.Err }
