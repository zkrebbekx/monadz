package monadz

import "sync"

// Lazy is a value that is computed on the first Get, and then cached.
// Get is safe for concurrent use; the function runs at most one time.
// If the function panics, every Get panics with the same value.
//
// Equivalents: Rust LazyLock, Haskell lazy evaluation.
type Lazy[T any] struct {
	get func() T
}

// NewLazy returns a Lazy that computes f on the first Get.
func NewLazy[T any](f func() T) Lazy[T] { return Lazy[T]{get: sync.OnceValue(f)} }

// LazyOf returns a Lazy for a function that can fail. The error is cached
// too, so a failed computation does not run again.
func LazyOf[T any](f func() (T, error)) Lazy[Result[T]] {
	g := sync.OnceValues(f)
	return Lazy[Result[T]]{get: func() Result[T] { return Of(g()) }}
}

// Get computes the value on the first call, and returns the cached value
// after that.
func (l Lazy[T]) Get() T {
	if l.get == nil {
		panic("monadz: Get on a zero Lazy; use NewLazy")
	}
	return l.get()
}

// Map returns a Lazy that applies f to this value. Nothing runs until Get.
func (l Lazy[T]) Map[U any](f func(T) U) Lazy[U] {
	return NewLazy(func() U { return f(l.Get()) })
}

// AndThen returns a Lazy that applies f to this value and gets the result.
func (l Lazy[T]) AndThen[U any](f func(T) Lazy[U]) Lazy[U] {
	return NewLazy(func() U { return f(l.Get()).Get() })
}

// ZipWith returns a Lazy that combines both values with f.
func (l Lazy[T]) ZipWith[U, V any](other Lazy[U], f func(T, U) V) Lazy[V] {
	return NewLazy(func() V { return f(l.Get(), other.Get()) })
}
