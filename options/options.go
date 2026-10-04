// Package options has functions that work on many lift.Option values, or
// that cannot be methods because of Go's type rules.
package options

import (
	"cmp"
	"iter"

	"github.com/zkrebbekx/lift"
)

// Zip returns Some of both values if both are Some, else None.
//
// Zip is a function, not a method: in Go 1.27 a method of Option[T] cannot
// return Option[Pair[T, U]] ("instantiation cycle").
func Zip[A, B any](a lift.Option[A], b lift.Option[B]) lift.Option[lift.Pair[A, B]] {
	return a.ZipWith(b, lift.PairOf[A, B])
}

// Zip3 returns Some of all three values if all are Some, else None.
func Zip3[A, B, C any](a lift.Option[A], b lift.Option[B], c lift.Option[C]) lift.Option[lift.Triple[A, B, C]] {
	return Map3(a, b, c, lift.TripleOf[A, B, C])
}

// Map3 combines three values with f if all are Some. (Haskell: liftA3.)
func Map3[A, B, C, R any](a lift.Option[A], b lift.Option[B], c lift.Option[C], f func(A, B, C) R) lift.Option[R] {
	va, okA := a.Get()
	vb, okB := b.Get()
	vc, okC := c.Get()
	return lift.WhenFunc(okA && okB && okC, func() R { return f(va, vb, vc) })
}

// Unzip splits Some(Pair) into two Somes, and None into two Nones.
func Unzip[A, B any](o lift.Option[lift.Pair[A, B]]) (lift.Option[A], lift.Option[B]) {
	p, ok := o.Get()
	return lift.FromTuple(p.First, ok), lift.FromTuple(p.Second, ok)
}

// Flatten removes one level of nesting. (Haskell: join.)
func Flatten[T any](o lift.Option[lift.Option[T]]) lift.Option[T] {
	inner, ok := o.Get()
	if !ok {
		return lift.None[T]()
	}
	return inner
}

// Transpose turns Option[Result[T]] into Result[Option[T]]:
// None -> Ok(None), Some(Ok(v)) -> Ok(Some(v)), Some(Err(e)) -> Err(e).
// (Rust: Option::transpose.)
func Transpose[T any](o lift.Option[lift.Result[T]]) lift.Result[lift.Option[T]] {
	r, ok := o.Get()
	if !ok {
		return lift.Ok(lift.None[T]())
	}
	v, err := r.Get()
	if err != nil {
		return lift.Err[lift.Option[T]](err)
	}
	return lift.Ok(lift.Some(v))
}

// Collect returns Some of all values if every Option is Some. It stops at
// the first None. (Rust: collect::<Option<Vec<_>>>; Haskell: sequence.)
func Collect[T any](seq iter.Seq[lift.Option[T]]) lift.Option[[]T] {
	out := []T{}
	for o := range seq {
		v, ok := o.Get()
		if !ok {
			return lift.None[[]T]()
		}
		out = append(out, v)
	}
	return lift.Some(out)
}

// Traverse applies f to each item, and returns Some of all results if every
// call returns Some. It stops at the first None. (Haskell: traverse.)
func Traverse[A, B any](seq iter.Seq[A], f func(A) lift.Option[B]) lift.Option[[]B] {
	out := []B{}
	for a := range seq {
		v, ok := f(a).Get()
		if !ok {
			return lift.None[[]B]()
		}
		out = append(out, v)
	}
	return lift.Some(out)
}

// Values yields the value of each Some and skips each None.
// (Haskell: catMaybes; Rust: flatten.)
func Values[T any](seq iter.Seq[lift.Option[T]]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for o := range seq {
			if v, ok := o.Get(); ok && !yield(v) {
				return
			}
		}
	}
}

// FilterMap applies f to each item and yields the values of the Somes.
// (Haskell: mapMaybe; Rust: filter_map.)
func FilterMap[A, B any](seq iter.Seq[A], f func(A) lift.Option[B]) iter.Seq[B] {
	return func(yield func(B) bool) {
		for a := range seq {
			if v, ok := f(a).Get(); ok && !yield(v) {
				return
			}
		}
	}
}

// First returns the first Some, or None. (Haskell: asum / <|>.)
func First[T any](opts ...lift.Option[T]) lift.Option[T] {
	for _, o := range opts {
		if o.IsSome() {
			return o
		}
	}
	return lift.None[T]()
}

// Compose joins two Option-returning steps into one. (Haskell: >=>.)
func Compose[A, B, C any](f func(A) lift.Option[B], g func(B) lift.Option[C]) func(A) lift.Option[C] {
	return func(a A) lift.Option[C] { return f(a).AndThen(g) }
}

// Equal reports whether both Options are None, or both hold equal values.
func Equal[T comparable](a, b lift.Option[T]) bool {
	va, okA := a.Get()
	vb, okB := b.Get()
	return okA == okB && va == vb
}

// Contains reports whether o is Some(v).
func Contains[T comparable](o lift.Option[T], v T) bool {
	got, ok := o.Get()
	return ok && got == v
}

// Find returns the first item that satisfies pred, or None.
// (Haskell: find; Rust: Iterator::find.)
func Find[T any](seq iter.Seq[T], pred func(T) bool) lift.Option[T] {
	for v := range seq {
		if pred(v) {
			return lift.Some(v)
		}
	}
	return lift.None[T]()
}

// Head returns the first item, or None for an empty sequence.
func Head[T any](seq iter.Seq[T]) lift.Option[T] {
	for v := range seq {
		return lift.Some(v)
	}
	return lift.None[T]()
}

// Last returns the last item, or None for an empty sequence.
func Last[T any](seq iter.Seq[T]) lift.Option[T] {
	out := lift.None[T]()
	for v := range seq {
		out = lift.Some(v)
	}
	return out
}

// Max returns the largest item, or None for an empty sequence. Unlike
// slices.Max, it does not panic on an empty input.
func Max[T cmp.Ordered](seq iter.Seq[T]) lift.Option[T] {
	return reduce(seq, func(a, b T) T { return max(a, b) })
}

// Min returns the smallest item, or None for an empty sequence.
func Min[T cmp.Ordered](seq iter.Seq[T]) lift.Option[T] {
	return reduce(seq, func(a, b T) T { return min(a, b) })
}

func reduce[T any](seq iter.Seq[T], f func(T, T) T) lift.Option[T] {
	var (
		acc  T
		seen bool
	)
	for v := range seq {
		if !seen {
			acc, seen = v, true
			continue
		}
		acc = f(acc, v)
	}
	return lift.FromTuple(acc, seen)
}
