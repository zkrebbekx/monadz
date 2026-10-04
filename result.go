package monadz

import (
	"fmt"
	"iter"
	"log/slog"
)

// Result holds a value of type T (Ok) or an error (Err).
//
// The zero value is Ok(zero T), which matches Go's (zero, nil) convention.
// Equivalents: Rust Result<T, E> with E = error, Haskell Either error a.
// For a typed failure value, use Either.
type Result[T any] struct {
	value T
	err   error
}

// Ok returns a successful Result.
func Ok[T any](v T) Result[T] { return Result[T]{value: v} }

// Err returns a failed Result. A nil err becomes ErrNilError, so Err never
// returns a success by mistake.
func Err[T any](err error) Result[T] { return Result[T]{err: nonNil(err)} }

// Errf returns a failed Result with fmt.Errorf(format, args...).
func Errf[T any](format string, args ...any) Result[T] {
	return Err[T](fmt.Errorf(format, args...))
}

// Of converts a Go (value, error) pair: monadz.Of(strconv.Atoi(s)).
func Of[T any](v T, err error) Result[T] {
	if err != nil {
		return failed[T](err)
	}
	return Ok(v)
}

// FromErr converts a function that returns only an error:
// monadz.FromErr(db.PingContext(ctx)).
func FromErr(err error) Result[Unit] { return Of(Unit{}, err) }

// ---- Queries ----

// IsOk reports whether the Result is a success.
func (r Result[T]) IsOk() bool { return r.err == nil }

// IsErr reports whether the Result is a failure.
func (r Result[T]) IsErr() bool { return r.err != nil }

// IsOkAnd reports whether the Result is a success with a value that
// satisfies pred.
func (r Result[T]) IsOkAnd(pred func(T) bool) bool { return r.err == nil && pred(r.value) }

// IsErrAnd reports whether the Result is a failure with an error that
// satisfies pred.
func (r Result[T]) IsErrAnd(pred func(error) bool) bool { return r.err != nil && pred(r.err) }

// Get returns the Go pair (value, error).
func (r Result[T]) Get() (T, error) {
	if r.err != nil {
		var zero T
		return zero, r.err
	}
	return r.value, nil
}

// Err returns the error, or nil for a success.
func (r Result[T]) Err() error { return r.err }

// Ok converts the Result to an Option, and discards the error. (Rust: ok.)
func (r Result[T]) Ok() Option[T] { return FromTuple(r.value, r.err == nil) }

// ---- Extraction ----

// Must returns the value, or panics with the error. (Rust: unwrap.)
func (r Result[T]) Must() T {
	if r.err != nil {
		panic(r.err)
	}
	return r.value
}

// Expect returns the value, or panics with msg and the error. (Rust: expect.)
func (r Result[T]) Expect(msg string) T {
	if r.err != nil {
		panic(fmt.Errorf("%s: %w", msg, r.err))
	}
	return r.value
}

// MustErr returns the error, or panics for a success. (Rust: unwrap_err.)
func (r Result[T]) MustErr() error {
	if r.err == nil {
		panic(fmt.Sprintf("monadz: MustErr on Ok(%v)", r.value))
	}
	return r.err
}

// ValueOr returns the value, or fallback for a failure. (Rust: unwrap_or.)
func (r Result[T]) ValueOr(fallback T) T {
	if r.err != nil {
		return fallback
	}
	return r.value
}

// ValueOrElse returns the value, or f(err). (Rust: unwrap_or_else.)
func (r Result[T]) ValueOrElse(f func(error) T) T {
	if r.err != nil {
		return f(r.err)
	}
	return r.value
}

// ValueOrZero returns the value, or the zero value of T.
// (Rust: unwrap_or_default.)
func (r Result[T]) ValueOrZero() T {
	if r.err != nil {
		var zero T
		return zero
	}
	return r.value
}

// Try returns the value. On a failure, it stops the enclosing Do, and Do
// returns the error. See Do.
func (r Result[T]) Try() T {
	if r.err != nil {
		panic(&earlyErr{r.err})
	}
	return r.value
}

// ---- Transformation ----

// Map applies f to the value. (Rust: map; Haskell: fmap.)
func (r Result[T]) Map[U any](f func(T) U) Result[U] {
	if r.err != nil {
		return failed[U](r.err)
	}
	return Ok(f(r.value))
}

// Then applies a Go-shaped step func(T) (U, error) to the value.
// strconv.Atoi, url.Parse, json.Marshal, and your own functions plug in
// directly.
func (r Result[T]) Then[U any](f func(T) (U, error)) Result[U] {
	if r.err != nil {
		return failed[U](r.err)
	}
	return Of(f(r.value))
}

// AndThen applies f to the value and returns its Result. This is the monad
// bind. (Rust: and_then; Haskell: >>=.)
func (r Result[T]) AndThen[U any](f func(T) Result[U]) Result[U] {
	if r.err != nil {
		return failed[U](r.err)
	}
	return f(r.value)
}

// MapErr applies f to the error. (Rust: map_err.)
func (r Result[T]) MapErr(f func(error) error) Result[T] {
	if r.err == nil {
		return r
	}
	return Err[T](f(r.err))
}

// Wrap adds context to the error: "msg: <err>". The original error stays
// reachable with errors.Is and errors.As. (Rust anyhow: context.)
func (r Result[T]) Wrap(msg string) Result[T] {
	if r.err == nil {
		return r
	}
	return failed[T](fmt.Errorf("%s: %w", msg, r.err))
}

// Wrapf is Wrap with a format string.
func (r Result[T]) Wrapf(format string, args ...any) Result[T] {
	if r.err == nil {
		return r
	}
	return r.Wrap(fmt.Sprintf(format, args...))
}

// Ensure returns Err(err) if the value does not satisfy pred.
// (Scala: filterOrElse; Haskell: guard.)
func (r Result[T]) Ensure(pred func(T) bool, err error) Result[T] {
	if r.err == nil && !pred(r.value) {
		return Err[T](err)
	}
	return r
}

// MapOr applies f to the value, or returns fallback. (Rust: map_or.)
func (r Result[T]) MapOr[U any](fallback U, f func(T) U) U {
	if r.err != nil {
		return fallback
	}
	return f(r.value)
}

// MapOrElse applies onOk to the value, or onErr to the error.
// (Rust: map_or_else.)
func (r Result[T]) MapOrElse[U any](onErr func(error) U, onOk func(T) U) U {
	if r.err != nil {
		return onErr(r.err)
	}
	return onOk(r.value)
}

// Match calls onOk with the value or onErr with the error, and returns the
// result. Unlike mo's result.Match, onErr receives the error.
// (Haskell: either.)
func (r Result[T]) Match[U any](onOk func(T) U, onErr func(error) U) U {
	if r.err != nil {
		return onErr(r.err)
	}
	return onOk(r.value)
}

// Inspect calls f with the value, if any, and returns the Result unchanged.
func (r Result[T]) Inspect(f func(T)) Result[T] {
	if r.err == nil {
		f(r.value)
	}
	return r
}

// InspectErr calls f with the error, if any, and returns the Result
// unchanged. Use it to log a failure in the middle of a chain.
func (r Result[T]) InspectErr(f func(error)) Result[T] {
	if r.err != nil {
		f(r.err)
	}
	return r
}

// ---- Recovery ----

// Or returns the Result if it is Ok, else other. (Rust: or.)
func (r Result[T]) Or(other Result[T]) Result[T] {
	if r.err == nil {
		return r
	}
	return other
}

// OrElse returns the Result if it is Ok, else f(err). (Rust: or_else.)
func (r Result[T]) OrElse(f func(error) Result[T]) Result[T] {
	if r.err == nil {
		return r
	}
	return f(r.err)
}

// Recover is OrElse with the Go shape func(error) (T, error). Return a
// value to recover, or return an error to stay on the failure track.
func (r Result[T]) Recover(f func(error) (T, error)) Result[T] {
	if r.err == nil {
		return r
	}
	return Of(f(r.err))
}

// ---- Combination ----

// And returns other if the Result is Ok, else the failure. (Rust: and.)
func (r Result[T]) And[U any](other Result[U]) Result[U] {
	if r.err != nil {
		return failed[U](r.err)
	}
	return other
}

// ZipWith combines both values with f if both are Ok, else the first
// failure. (Haskell: liftA2.)
func (r Result[T]) ZipWith[U, V any](other Result[U], f func(T, U) V) Result[V] {
	switch {
	case r.err != nil:
		return failed[V](r.err)
	case other.err != nil:
		return failed[V](other.err)
	default:
		return Ok(f(r.value, other.value))
	}
}

// ---- Conversion ----

// Either converts the Result to Either[error, T].
func (r Result[T]) Either() Either[error, T] {
	if r.err != nil {
		return Left[error, T](r.err)
	}
	return Right[error](r.value)
}

// Validation converts the Result to a Validation, so that it can collect
// errors together with other Validations.
func (r Result[T]) Validation() Validation[T] {
	if r.err != nil {
		return Invalid[T](r.err)
	}
	return Valid(r.value)
}

// All returns an iterator over the value: one element for Ok, none for Err.
func (r Result[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		if r.err == nil {
			yield(r.value)
		}
	}
}

// ---- Formatting ----

// String returns "Ok(<value>)" or "Err(<error>)".
func (r Result[T]) String() string {
	if r.err != nil {
		return fmt.Sprintf("Err(%v)", r.err)
	}
	return fmt.Sprintf("Ok(%v)", r.value)
}

// LogValue makes slog log the value, or a group with the error message.
func (r Result[T]) LogValue() slog.Value {
	if r.err != nil {
		return slog.GroupValue(slog.String("error", r.err.Error()))
	}
	return slog.AnyValue(r.value)
}

// failed returns a failed Result without the nil check of Err. Methods use
// it to pass an existing (non-nil) error along.
func failed[T any](err error) Result[T] { return Result[T]{err: err} }
