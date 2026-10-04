// Package results has functions that work on many lift.Result values, or
// that cannot be methods because of Go's type rules.
package results

import (
	"errors"
	"iter"

	"github.com/zkrebbekx/lift"
)

// Zip returns Ok of both values if both are Ok, else the first failure.
//
// Zip is a function, not a method: in Go 1.27 a method of Result[T] cannot
// return Result[Pair[T, U]] ("instantiation cycle").
func Zip[A, B any](a lift.Result[A], b lift.Result[B]) lift.Result[lift.Pair[A, B]] {
	return a.ZipWith(b, lift.PairOf[A, B])
}

// Zip3 returns Ok of all three values if all are Ok, else the first failure.
func Zip3[A, B, C any](a lift.Result[A], b lift.Result[B], c lift.Result[C]) lift.Result[lift.Triple[A, B, C]] {
	return Map3(a, b, c, lift.TripleOf[A, B, C])
}

// Map3 combines three values with f if all are Ok, else the first failure.
// (Haskell: liftA3.)
func Map3[A, B, C, R any](a lift.Result[A], b lift.Result[B], c lift.Result[C], f func(A, B, C) R) lift.Result[R] {
	va, err := a.Get()
	if err != nil {
		return lift.Err[R](err)
	}
	vb, err := b.Get()
	if err != nil {
		return lift.Err[R](err)
	}
	vc, err := c.Get()
	if err != nil {
		return lift.Err[R](err)
	}
	return lift.Ok(f(va, vb, vc))
}

// Flatten removes one level of nesting. (Haskell: join.)
func Flatten[T any](r lift.Result[lift.Result[T]]) lift.Result[T] {
	inner, err := r.Get()
	if err != nil {
		return lift.Err[T](err)
	}
	return inner
}

// Transpose turns Result[Option[T]] into Option[Result[T]]:
// Ok(None) -> None, Ok(Some(v)) -> Some(Ok(v)), Err(e) -> Some(Err(e)).
// (Rust: Result::transpose.)
func Transpose[T any](r lift.Result[lift.Option[T]]) lift.Option[lift.Result[T]] {
	o, err := r.Get()
	if err != nil {
		return lift.Some(lift.Err[T](err))
	}
	v, ok := o.Get()
	if !ok {
		return lift.None[lift.Result[T]]()
	}
	return lift.Some(lift.Ok(v))
}

// Lift converts a Go-shaped function into a Result-returning function.
func Lift[A, B any](f func(A) (B, error)) func(A) lift.Result[B] {
	return func(a A) lift.Result[B] { return lift.Of(f(a)) }
}

// Compose joins two Result-returning steps into one. (Haskell: >=>.)
func Compose[A, B, C any](f func(A) lift.Result[B], g func(B) lift.Result[C]) func(A) lift.Result[C] {
	return func(a A) lift.Result[C] { return f(a).AndThen(g) }
}

// Collect returns Ok of all values if every Result is Ok. It stops at the
// first failure and does not read the rest of the sequence.
// (Rust: collect::<Result<Vec<_>, _>>; Haskell: sequence.)
func Collect[T any](seq iter.Seq[lift.Result[T]]) lift.Result[[]T] {
	out := []T{}
	for r := range seq {
		v, err := r.Get()
		if err != nil {
			return lift.Err[[]T](err)
		}
		out = append(out, v)
	}
	return lift.Ok(out)
}

// FromSeq2 collects a Go (value, error) sequence, such as a database row
// iterator. It stops at the first error.
func FromSeq2[T any](seq iter.Seq2[T, error]) lift.Result[[]T] {
	out := []T{}
	for v, err := range seq {
		if err != nil {
			return lift.Err[[]T](err)
		}
		out = append(out, v)
	}
	return lift.Ok(out)
}

// Seq converts a Go (value, error) sequence into a sequence of Results.
func Seq[T any](seq iter.Seq2[T, error]) iter.Seq[lift.Result[T]] {
	return func(yield func(lift.Result[T]) bool) {
		for v, err := range seq {
			if !yield(lift.Of(v, err)) {
				return
			}
		}
	}
}

// Traverse applies f to each item and returns Ok of all results, or the
// first failure. (Haskell: traverse / mapM.)
func Traverse[A, B any](seq iter.Seq[A], f func(A) lift.Result[B]) lift.Result[[]B] {
	out := []B{}
	for a := range seq {
		v, err := f(a).Get()
		if err != nil {
			return lift.Err[[]B](err)
		}
		out = append(out, v)
	}
	return lift.Ok(out)
}

// TraverseFunc is Traverse for a Go-shaped function func(A) (B, error).
func TraverseFunc[A, B any](seq iter.Seq[A], f func(A) (B, error)) lift.Result[[]B] {
	return Traverse(seq, Lift(f))
}

// Fold runs f over the items with an accumulator, and stops at the first
// failure. (Haskell: foldM.)
func Fold[A, B any](seq iter.Seq[A], init B, f func(B, A) lift.Result[B]) lift.Result[B] {
	acc := init
	for a := range seq {
		next, err := f(acc, a).Get()
		if err != nil {
			return lift.Err[B](err)
		}
		acc = next
	}
	return lift.Ok(acc)
}

// Partition splits the Results into the values and the errors, in order.
// (Haskell: partitionEithers.)
func Partition[T any](seq iter.Seq[lift.Result[T]]) ([]T, []error) {
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
func Oks[T any](seq iter.Seq[lift.Result[T]]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for r := range seq {
			if v, err := r.Get(); err == nil && !yield(v) {
				return
			}
		}
	}
}

// Errs yields the error of each Err and skips each Ok.
func Errs[T any](seq iter.Seq[lift.Result[T]]) iter.Seq[error] {
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
func First[T any](rs ...lift.Result[T]) lift.Result[T] {
	errs := make([]error, 0, len(rs))
	for _, r := range rs {
		if r.IsOk() {
			return r
		}
		errs = append(errs, r.Err())
	}
	if len(errs) == 0 {
		return lift.Err[T](errors.New("results: First with no arguments"))
	}
	return lift.Err[T](errors.Join(errs...))
}
