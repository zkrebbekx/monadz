// Package results has functions that work on many monadz.Result values, or
// that cannot be methods because of Go's type rules.
//
// The functions fall into three groups:
//
//   - Combine: Zip, Zip3, and Map3 join Results and stop at the first error.
//     First returns the first Ok. Compose joins two Result-returning
//     functions. FromFunc changes a func(A) (B, error) into a
//     func(A) Result[B].
//   - Collect: Collect, Traverse, TraverseFunc, and Fold return Ok of every
//     value, or the first error. FromSeq2 collects a (value, error)
//     iterator, such as rows from a database.
//   - Split: Partition, Oks, and Errs separate the values from the errors.
//
// Flatten and Transpose are functions, not methods, because a method
// receiver cannot be Result[Result[T]].
//
// The collection functions take an iter.Seq, so slices.Values works directly:
//
//	ports := results.TraverseFunc(slices.Values([]string{"80", "443"}), strconv.Atoi)
//	// Ok([80 443]); with "x" in the list, the result is the Atoi error.
package results

import (
	"errors"
	"iter"

	"github.com/zkrebbekx/monadz"
)

// Zip returns Ok of both values if both are Ok, else the first failure.
//
// Zip is a function, not a method: in Go 1.27 a method of Result[T] cannot
// return Result[Pair[T, U]] ("instantiation cycle").
func Zip[A, B any](a monadz.Result[A], b monadz.Result[B]) monadz.Result[monadz.Pair[A, B]] {
	return a.ZipWith(b, monadz.PairOf[A, B])
}

// Zip3 returns Ok of all three values if all are Ok, else the first failure.
func Zip3[A, B, C any](a monadz.Result[A], b monadz.Result[B], c monadz.Result[C]) monadz.Result[monadz.Triple[A, B, C]] {
	return Map3(a, b, c, monadz.TripleOf[A, B, C])
}

// Map3 combines three values with f if all are Ok, else the first failure.
// (Haskell: liftA3.)
func Map3[A, B, C, R any](a monadz.Result[A], b monadz.Result[B], c monadz.Result[C], f func(A, B, C) R) monadz.Result[R] {
	va, err := a.Get()
	if err != nil {
		return monadz.Err[R](err)
	}
	vb, err := b.Get()
	if err != nil {
		return monadz.Err[R](err)
	}
	vc, err := c.Get()
	if err != nil {
		return monadz.Err[R](err)
	}
	return monadz.Ok(f(va, vb, vc))
}

// Flatten removes one level of nesting. (Haskell: join.)
func Flatten[T any](r monadz.Result[monadz.Result[T]]) monadz.Result[T] {
	inner, err := r.Get()
	if err != nil {
		return monadz.Err[T](err)
	}
	return inner
}

// Transpose turns Result[Option[T]] into Option[Result[T]]:
// Ok(None) -> None, Ok(Some(v)) -> Some(Ok(v)), Err(e) -> Some(Err(e)).
// (Rust: Result::transpose.)
func Transpose[T any](r monadz.Result[monadz.Option[T]]) monadz.Option[monadz.Result[T]] {
	o, err := r.Get()
	if err != nil {
		return monadz.Some(monadz.Err[T](err))
	}
	v, ok := o.Get()
	if !ok {
		return monadz.None[monadz.Result[T]]()
	}
	return monadz.Some(monadz.Ok(v))
}

// FromFunc converts a Go-shaped function func(A) (B, error) into a
// Result-returning function.
func FromFunc[A, B any](f func(A) (B, error)) func(A) monadz.Result[B] {
	return func(a A) monadz.Result[B] { return monadz.Of(f(a)) }
}

// Compose joins two Result-returning steps into one. (Haskell: >=>.)
func Compose[A, B, C any](f func(A) monadz.Result[B], g func(B) monadz.Result[C]) func(A) monadz.Result[C] {
	return func(a A) monadz.Result[C] { return f(a).AndThen(g) }
}

// Collect returns Ok of all values if every Result is Ok. It stops at the
// first failure and does not read the rest of the sequence.
// (Rust: collect::<Result<Vec<_>, _>>; Haskell: sequence.)
func Collect[T any](seq iter.Seq[monadz.Result[T]]) monadz.Result[[]T] {
	return Traverse(seq, identity[monadz.Result[T]])
}

func identity[T any](v T) T { return v }

// FromSeq2 collects a Go (value, error) sequence, such as a database row
// iterator. It stops at the first error.
func FromSeq2[T any](seq iter.Seq2[T, error]) monadz.Result[[]T] {
	return Collect(Seq(seq))
}

// Seq converts a Go (value, error) sequence into a sequence of Results.
func Seq[T any](seq iter.Seq2[T, error]) iter.Seq[monadz.Result[T]] {
	return func(yield func(monadz.Result[T]) bool) {
		for v, err := range seq {
			if !yield(monadz.Of(v, err)) {
				return
			}
		}
	}
}

// Traverse applies f to each item and returns Ok of all results, or the
// first failure. (Haskell: traverse / mapM.)
func Traverse[A, B any](seq iter.Seq[A], f func(A) monadz.Result[B]) monadz.Result[[]B] {
	out := []B{}
	for a := range seq {
		v, err := f(a).Get()
		if err != nil {
			return monadz.Err[[]B](err)
		}
		out = append(out, v)
	}
	return monadz.Ok(out)
}

// TraverseFunc is Traverse for a Go-shaped function func(A) (B, error).
func TraverseFunc[A, B any](seq iter.Seq[A], f func(A) (B, error)) monadz.Result[[]B] {
	return Traverse(seq, FromFunc(f))
}

// Fold runs f over the items with an accumulator, and stops at the first
// failure. (Haskell: foldM.)
func Fold[A, B any](seq iter.Seq[A], init B, f func(B, A) monadz.Result[B]) monadz.Result[B] {
	acc := init
	for a := range seq {
		next, err := f(acc, a).Get()
		if err != nil {
			return monadz.Err[B](err)
		}
		acc = next
	}
	return monadz.Ok(acc)
}

// Partition splits the Results into the values and the errors, in order.
// (Haskell: partitionEithers.)
func Partition[T any](seq iter.Seq[monadz.Result[T]]) ([]T, []error) {
	var oks []T
	var errs []error
	for r := range seq {
		if v, err := r.Get(); err != nil {
			errs = append(errs, err)
		} else {
			oks = append(oks, v)
		}
	}
	return oks, errs
}

// Oks yields the value of each Ok and skips each Err.
func Oks[T any](seq iter.Seq[monadz.Result[T]]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for r := range seq {
			if v, err := r.Get(); err == nil && !yield(v) {
				return
			}
		}
	}
}

// Errs yields the error of each Err and skips each Ok.
func Errs[T any](seq iter.Seq[monadz.Result[T]]) iter.Seq[error] {
	return func(yield func(error) bool) {
		for r := range seq {
			if err := r.Err(); err != nil && !yield(err) {
				return
			}
		}
	}
}

// First returns the first Ok. If no Result is Ok, it returns all the errors
// joined with errors.Join. (Haskell: asum.)
func First[T any](rs ...monadz.Result[T]) monadz.Result[T] {
	errs := make([]error, 0, len(rs))
	for _, r := range rs {
		if r.IsOk() {
			return r
		}
		errs = append(errs, r.Err())
	}
	if len(errs) == 0 {
		return monadz.Err[T](errors.New("results: First with no arguments"))
	}
	return monadz.Err[T](errors.Join(errs...))
}
