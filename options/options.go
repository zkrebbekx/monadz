// Package options has functions that work on many monadz.Option values, or
// that cannot be methods because of Go's type rules.
//
// The functions fall into three groups:
//
//   - Combine: Zip, Zip3, Map3, and Unzip join or split Options. First
//     returns the first Some. Compose joins two Option-returning functions.
//   - Collect: Collect and Traverse return Some of every value, or None if
//     one value is None. Values and FilterMap keep only the Some values.
//   - Search: Find, Head, Last, Min, and Max return an Option instead of a
//     value and a bool.
//
// Flatten and Transpose are functions, not methods, because a method
// receiver cannot be Option[Option[T]].
//
// The collection functions take an iter.Seq, so slices.Values and maps.Keys
// work directly:
//
//	ports := options.Traverse(slices.Values(names), func(n string) monadz.Option[int] {
//		return monadz.Lookup(portOf, n)
//	})
//	// Some([80 443]) if every name has a port; None if one name has none.
package options

import (
	"cmp"
	"iter"

	"github.com/zkrebbekx/monadz"
)

// Zip returns Some of both values if both are Some, else None.
//
// Zip is a function, not a method: in Go 1.27 a method of Option[T] cannot
// return Option[Pair[T, U]] ("instantiation cycle").
func Zip[A, B any](a monadz.Option[A], b monadz.Option[B]) monadz.Option[monadz.Pair[A, B]] {
	return a.ZipWith(b, monadz.PairOf[A, B])
}

// Zip3 returns Some of all three values if all are Some, else None.
func Zip3[A, B, C any](a monadz.Option[A], b monadz.Option[B], c monadz.Option[C]) monadz.Option[monadz.Triple[A, B, C]] {
	return Map3(a, b, c, monadz.TripleOf[A, B, C])
}

// Map3 combines three values with f if all are Some. (Haskell: liftA3.)
func Map3[A, B, C, R any](a monadz.Option[A], b monadz.Option[B], c monadz.Option[C], f func(A, B, C) R) monadz.Option[R] {
	va, okA := a.Get()
	vb, okB := b.Get()
	vc, okC := c.Get()
	return monadz.WhenFunc(okA && okB && okC, func() R { return f(va, vb, vc) })
}

// Unzip splits Some(Pair) into two Somes, and None into two Nones.
func Unzip[A, B any](o monadz.Option[monadz.Pair[A, B]]) (monadz.Option[A], monadz.Option[B]) {
	p, ok := o.Get()
	return monadz.FromTuple(p.First, ok), monadz.FromTuple(p.Second, ok)
}

// Flatten removes one level of nesting. (Haskell: join.)
func Flatten[T any](o monadz.Option[monadz.Option[T]]) monadz.Option[T] {
	inner, ok := o.Get()
	if !ok {
		return monadz.None[T]()
	}
	return inner
}

// Transpose turns Option[Result[T]] into Result[Option[T]]:
// None -> Ok(None), Some(Ok(v)) -> Ok(Some(v)), Some(Err(e)) -> Err(e).
// (Rust: Option::transpose.)
func Transpose[T any](o monadz.Option[monadz.Result[T]]) monadz.Result[monadz.Option[T]] {
	r, ok := o.Get()
	if !ok {
		return monadz.Ok(monadz.None[T]())
	}
	v, err := r.Get()
	if err != nil {
		return monadz.Err[monadz.Option[T]](err)
	}
	return monadz.Ok(monadz.Some(v))
}

// Collect returns Some of all values if every Option is Some. It stops at
// the first None. (Rust: collect::<Option<Vec<_>>>; Haskell: sequence.)
func Collect[T any](seq iter.Seq[monadz.Option[T]]) monadz.Option[[]T] {
	return Traverse(seq, identity[monadz.Option[T]])
}

// Traverse applies f to each item, and returns Some of all results if every
// call returns Some. It stops at the first None. (Haskell: traverse.)
func Traverse[A, B any](seq iter.Seq[A], f func(A) monadz.Option[B]) monadz.Option[[]B] {
	out := []B{}
	for a := range seq {
		v, ok := f(a).Get()
		if !ok {
			return monadz.None[[]B]()
		}
		out = append(out, v)
	}
	return monadz.Some(out)
}

// Values yields the value of each Some and skips each None.
// (Haskell: catMaybes; Rust: flatten.)
func Values[T any](seq iter.Seq[monadz.Option[T]]) iter.Seq[T] {
	return FilterMap(seq, identity[monadz.Option[T]])
}

// FilterMap applies f to each item and yields the values of the Somes.
// (Haskell: mapMaybe; Rust: filter_map.)
func FilterMap[A, B any](seq iter.Seq[A], f func(A) monadz.Option[B]) iter.Seq[B] {
	return func(yield func(B) bool) {
		for a := range seq {
			if v, ok := f(a).Get(); ok && !yield(v) {
				return
			}
		}
	}
}

// First returns the first Some, or None. (Haskell: asum / <|>.)
func First[T any](opts ...monadz.Option[T]) monadz.Option[T] {
	for _, o := range opts {
		if o.IsSome() {
			return o
		}
	}
	return monadz.None[T]()
}

// Compose joins two Option-returning steps into one. (Haskell: >=>.)
func Compose[A, B, C any](f func(A) monadz.Option[B], g func(B) monadz.Option[C]) func(A) monadz.Option[C] {
	return func(a A) monadz.Option[C] { return f(a).AndThen(g) }
}

// Equal reports whether both Options are None, or both hold equal values.
func Equal[T comparable](a, b monadz.Option[T]) bool {
	va, okA := a.Get()
	vb, okB := b.Get()
	return okA == okB && va == vb
}

// Contains reports whether o is Some(v).
func Contains[T comparable](o monadz.Option[T], v T) bool {
	got, ok := o.Get()
	return ok && got == v
}

// Find returns the first item that satisfies pred, or None.
// (Haskell: find; Rust: Iterator::find.)
func Find[T any](seq iter.Seq[T], pred func(T) bool) monadz.Option[T] {
	for v := range seq {
		if pred(v) {
			return monadz.Some(v)
		}
	}
	return monadz.None[T]()
}

// Head returns the first item, or None for an empty sequence.
func Head[T any](seq iter.Seq[T]) monadz.Option[T] {
	for v := range seq {
		return monadz.Some(v)
	}
	return monadz.None[T]()
}

// Last returns the last item, or None for an empty sequence.
func Last[T any](seq iter.Seq[T]) monadz.Option[T] {
	out := monadz.None[T]()
	for v := range seq {
		out = monadz.Some(v)
	}
	return out
}

// Max returns the largest item, or None for an empty sequence. Unlike
// slices.Max, it does not panic on an empty input.
func Max[T cmp.Ordered](seq iter.Seq[T]) monadz.Option[T] {
	return reduce(seq, func(a, b T) T { return max(a, b) })
}

// Min returns the smallest item, or None for an empty sequence.
func Min[T cmp.Ordered](seq iter.Seq[T]) monadz.Option[T] {
	return reduce(seq, func(a, b T) T { return min(a, b) })
}

func identity[T any](v T) T { return v }

func reduce[T any](seq iter.Seq[T], f func(T, T) T) monadz.Option[T] {
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
	return monadz.FromTuple(acc, seen)
}
