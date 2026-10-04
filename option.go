package monadz

import (
	"fmt"
	"iter"
	"log/slog"
)

// Option holds a value of type T (Some) or no value (None).
//
// The zero value is None. Equivalents: Rust Option<T>, Haskell Maybe a.
type Option[T any] struct {
	value T
	ok    bool
}

// Some returns an Option that holds v.
func Some[T any](v T) Option[T] { return Option[T]{value: v, ok: true} }

// None returns an Option with no value.
func None[T any]() Option[T] { return Option[T]{} }

// FromTuple converts a "comma ok" pair: FromTuple(m[k]) is not legal Go,
// so write v, ok := m[k]; FromTuple(v, ok). See also Lookup.
func FromTuple[T any](v T, ok bool) Option[T] {
	if !ok {
		return None[T]()
	}
	return Some(v)
}

// FromPtr returns None for a nil pointer, else Some of the pointed-to value.
func FromPtr[T any](p *T) Option[T] {
	if p == nil {
		return None[T]()
	}
	return Some(*p)
}

// FromZero returns None for the zero value of T, else Some(v).
// Use it for APIs where "" or 0 means "not set".
func FromZero[T comparable](v T) Option[T] {
	var zero T
	return FromTuple(v, v != zero)
}

// When returns Some(v) if cond is true, else None. (Rust: bool::then_some.)
func When[T any](cond bool, v T) Option[T] { return FromTuple(v, cond) }

// WhenFunc returns Some(f()) if cond is true, else None. f runs only when
// cond is true. (Rust: bool::then.)
func WhenFunc[T any](cond bool, f func() T) Option[T] {
	if !cond {
		return None[T]()
	}
	return Some(f())
}

// Lookup returns the value of key k in map m, or None.
func Lookup[K comparable, V any](m map[K]V, k K) Option[V] {
	v, ok := m[k]
	return FromTuple(v, ok)
}

// At returns s[i], or None if i is out of range. A negative i counts from
// the end: At(s, -1) is the last element. (Rust: slice::get.)
func At[T any](s []T, i int) Option[T] {
	if i < 0 {
		i += len(s)
	}
	if i < 0 || i >= len(s) {
		return None[T]()
	}
	return Some(s[i])
}

// ---- Queries ----

// IsSome reports whether the Option holds a value.
func (o Option[T]) IsSome() bool { return o.ok }

// IsNone reports whether the Option holds no value.
func (o Option[T]) IsNone() bool { return !o.ok }

// IsSomeAnd reports whether the Option holds a value that satisfies pred.
func (o Option[T]) IsSomeAnd(pred func(T) bool) bool { return o.ok && pred(o.value) }

// IsNoneOr reports whether the Option is None or holds a value that
// satisfies pred.
func (o Option[T]) IsNoneOr(pred func(T) bool) bool { return !o.ok || pred(o.value) }

// Get returns the value and true, or the zero value and false.
func (o Option[T]) Get() (T, bool) { return o.value, o.ok }

// ---- Extraction ----

// Must returns the value, or panics with ErrNone. (Rust: unwrap.)
func (o Option[T]) Must() T {
	if !o.ok {
		panic(ErrNone)
	}
	return o.value
}

// Expect returns the value, or panics with msg. (Rust: expect.)
func (o Option[T]) Expect(msg string) T {
	if !o.ok {
		panic(fmt.Errorf("%s: %w", msg, ErrNone))
	}
	return o.value
}

// ValueOr returns the value, or fallback. (Rust: unwrap_or; Haskell: fromMaybe.)
func (o Option[T]) ValueOr(fallback T) T {
	if !o.ok {
		return fallback
	}
	return o.value
}

// ValueOrElse returns the value, or the result of f. f runs only for None.
// (Rust: unwrap_or_else.)
func (o Option[T]) ValueOrElse(f func() T) T {
	if !o.ok {
		return f()
	}
	return o.value
}

// ValueOrZero returns the value, or the zero value of T.
// (Rust: unwrap_or_default.)
func (o Option[T]) ValueOrZero() T { return o.value }

// Ptr returns a pointer to a copy of the value, or nil.
func (o Option[T]) Ptr() *T {
	if !o.ok {
		return nil
	}
	v := o.value
	return &v
}

// Try returns the value. On None, it stops the enclosing Do or DoOption.
// See Do.
func (o Option[T]) Try() T {
	if !o.ok {
		panic(noneSignal)
	}
	return o.value
}

// ---- Transformation ----

// Map applies f to the value. (Rust: map; Haskell: fmap.)
func (o Option[T]) Map[U any](f func(T) U) Option[U] {
	if !o.ok {
		return None[U]()
	}
	return Some(f(o.value))
}

// AndThen applies f to the value and returns its Option. This is the monad
// bind. (Rust: and_then; Haskell: >>=.)
func (o Option[T]) AndThen[U any](f func(T) Option[U]) Option[U] {
	if !o.ok {
		return None[U]()
	}
	return f(o.value)
}

// Then applies a "comma ok" function to the value. A map lookup or a type
// assertion helper plugs in directly.
func (o Option[T]) Then[U any](f func(T) (U, bool)) Option[U] {
	if !o.ok {
		return None[U]()
	}
	return FromTuple(f(o.value))
}

// MapOr applies f to the value, or returns fallback. (Rust: map_or.)
func (o Option[T]) MapOr[U any](fallback U, f func(T) U) U {
	if !o.ok {
		return fallback
	}
	return f(o.value)
}

// MapOrElse applies f to the value, or returns fallback(). (Rust: map_or_else.)
func (o Option[T]) MapOrElse[U any](fallback func() U, f func(T) U) U {
	if !o.ok {
		return fallback()
	}
	return f(o.value)
}

// Match calls onSome with the value, or onNone, and returns the result.
// (Haskell: maybe.)
func (o Option[T]) Match[U any](onSome func(T) U, onNone func() U) U {
	if !o.ok {
		return onNone()
	}
	return onSome(o.value)
}

// Filter returns the Option if its value satisfies pred, else None.
func (o Option[T]) Filter(pred func(T) bool) Option[T] {
	if o.ok && pred(o.value) {
		return o
	}
	return None[T]()
}

// Inspect calls f with the value, if any, and returns the Option unchanged.
func (o Option[T]) Inspect(f func(T)) Option[T] {
	if o.ok {
		f(o.value)
	}
	return o
}

// ---- Combination ----

// And returns other if the Option is Some, else None. (Rust: and; Haskell: *>.)
func (o Option[T]) And[U any](other Option[U]) Option[U] {
	if !o.ok {
		return None[U]()
	}
	return other
}

// Or returns the Option if it is Some, else other. (Rust: or; Haskell: <|>.)
func (o Option[T]) Or(other Option[T]) Option[T] {
	if o.ok {
		return o
	}
	return other
}

// OrElse returns the Option if it is Some, else f(). f runs only for None.
func (o Option[T]) OrElse(f func() Option[T]) Option[T] {
	if o.ok {
		return o
	}
	return f()
}

// Xor returns the Option that is Some if exactly one of the two is Some,
// else None.
func (o Option[T]) Xor(other Option[T]) Option[T] {
	switch {
	case o.ok && !other.ok:
		return o
	case !o.ok && other.ok:
		return other
	default:
		return None[T]()
	}
}

// ZipWith combines both values with f if both are Some, else None.
// (Haskell: liftA2.)
func (o Option[T]) ZipWith[U, V any](other Option[U], f func(T, U) V) Option[V] {
	if !o.ok || !other.ok {
		return None[V]()
	}
	return Some(f(o.value, other.value))
}

// ---- Conversion ----

// OkOr converts Some(v) to Ok(v), and None to Err(err). (Rust: ok_or.)
func (o Option[T]) OkOr(err error) Result[T] {
	if !o.ok {
		return Err[T](err)
	}
	return Ok(o.value)
}

// OkOrElse converts Some(v) to Ok(v), and None to Err(f()). f runs only for
// None. (Rust: ok_or_else.)
func (o Option[T]) OkOrElse(f func() error) Result[T] {
	if !o.ok {
		return Err[T](f())
	}
	return Ok(o.value)
}

// All returns an iterator over the value: one element for Some, none for
// None. Use it in a range loop. (Rust: iter; Haskell: maybeToList.)
func (o Option[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		if o.ok {
			yield(o.value)
		}
	}
}

// ---- In-place changes (pointer receiver) ----

// Take returns the Option and leaves None in its place. (Rust: take.)
func (o *Option[T]) Take() Option[T] {
	old := *o
	*o = None[T]()
	return old
}

// TakeIf takes the value only if it satisfies pred. (Rust: take_if.)
func (o *Option[T]) TakeIf(pred func(T) bool) Option[T] {
	if o.ok && pred(o.value) {
		return o.Take()
	}
	return None[T]()
}

// Replace puts Some(v) in place and returns the old Option. (Rust: replace.)
func (o *Option[T]) Replace(v T) Option[T] {
	old := *o
	*o = Some(v)
	return old
}

// Insert puts Some(v) in place and returns a pointer to the stored value.
// (Rust: insert.)
func (o *Option[T]) Insert(v T) *T {
	*o = Some(v)
	return &o.value
}

// GetOrInsert stores v if the Option is None, and returns a pointer to the
// stored value. (Rust: get_or_insert.)
func (o *Option[T]) GetOrInsert(v T) *T {
	if !o.ok {
		*o = Some(v)
	}
	return &o.value
}

// GetOrInsertWith stores f() if the Option is None, and returns a pointer
// to the stored value. f runs only for None. (Rust: get_or_insert_with.)
func (o *Option[T]) GetOrInsertWith(f func() T) *T {
	if !o.ok {
		*o = Some(f())
	}
	return &o.value
}

// ---- Formatting ----

// String returns "Some(<value>)" or "None".
func (o Option[T]) String() string {
	if !o.ok {
		return "None"
	}
	return fmt.Sprintf("Some(%v)", o.value)
}

// LogValue makes slog log the value itself, or null for None.
func (o Option[T]) LogValue() slog.Value {
	if !o.ok {
		return slog.AnyValue(nil)
	}
	return slog.AnyValue(o.value)
}
