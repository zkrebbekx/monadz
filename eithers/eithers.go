// Package eithers has functions that work on many monadz.Either values.
package eithers

import (
	"iter"

	"github.com/zkrebbekx/monadz"
)

// Partition splits the Eithers into the Left values and the Right values,
// in order. (Haskell: partitionEithers.)
func Partition[L, R any](seq iter.Seq[monadz.Either[L, R]]) ([]L, []R) {
	var ls []L
	var rs []R
	for e := range seq {
		l, r, isRight := e.Unpack()
		if isRight {
			rs = append(rs, r)
		} else {
			ls = append(ls, l)
		}
	}
	return ls, rs
}

// Lefts yields the Left values. (Haskell: lefts.)
func Lefts[L, R any](seq iter.Seq[monadz.Either[L, R]]) iter.Seq[L] {
	return func(yield func(L) bool) {
		for e := range seq {
			if l, _, isRight := e.Unpack(); !isRight && !yield(l) {
				return
			}
		}
	}
}

// Rights yields the Right values. (Haskell: rights.)
func Rights[L, R any](seq iter.Seq[monadz.Either[L, R]]) iter.Seq[R] {
	return func(yield func(R) bool) {
		for e := range seq {
			if _, r, isRight := e.Unpack(); isRight && !yield(r) {
				return
			}
		}
	}
}

// Flatten removes one level of nesting on the Right side. (Haskell: join.)
func Flatten[L, R any](e monadz.Either[L, monadz.Either[L, R]]) monadz.Either[L, R] {
	l, inner, isRight := e.Unpack()
	if !isRight {
		return monadz.Left[L, R](l)
	}
	return inner
}

// ToResult converts Either[error, R] to Result[R].
func ToResult[R any](e monadz.Either[error, R]) monadz.Result[R] {
	l, r, isRight := e.Unpack()
	if !isRight {
		return monadz.Err[R](l)
	}
	return monadz.Ok(r)
}

// Collect returns Right of all values if every Either is Right. It stops at
// the first Left.
func Collect[L, R any](seq iter.Seq[monadz.Either[L, R]]) monadz.Either[L, []R] {
	out := []R{}
	for e := range seq {
		l, r, isRight := e.Unpack()
		if !isRight {
			return monadz.Left[L, []R](l)
		}
		out = append(out, r)
	}
	return monadz.Right[L](out)
}
