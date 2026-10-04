package lift

import "fmt"

// Either holds a value of type L (Left) or a value of type R (Right).
//
// Use Either when both outcomes are valid and carry their own data, for
// example Either[Paywall, Grant]. For "a value or an error", use Result.
//
// Either is right-biased: Map and AndThen work on the Right value, as in
// Haskell and Scala. The zero value is Left(zero L).
type Either[L, R any] struct {
	left    L
	right   R
	isRight bool
}

// Left returns an Either that holds the Left value l. Go cannot infer R, so
// write both type arguments: lift.Left[Paywall, Grant](p).
func Left[L, R any](l L) Either[L, R] { return Either[L, R]{left: l} }

// Right returns an Either that holds the Right value r. Go cannot infer L,
// so write it: lift.Right[Paywall](grant).
func Right[L, R any](r R) Either[L, R] { return Either[L, R]{right: r, isRight: true} }

// IsLeft reports whether the Either holds a Left value.
func (e Either[L, R]) IsLeft() bool { return !e.isRight }

// IsRight reports whether the Either holds a Right value.
func (e Either[L, R]) IsRight() bool { return e.isRight }

// IsLeftAnd reports whether the Either holds a Left value that satisfies pred.
func (e Either[L, R]) IsLeftAnd(pred func(L) bool) bool { return !e.isRight && pred(e.left) }

// IsRightAnd reports whether the Either holds a Right value that satisfies pred.
func (e Either[L, R]) IsRightAnd(pred func(R) bool) bool { return e.isRight && pred(e.right) }

// Left returns the Left value as an Option. (Haskell: fromLeft as Maybe.)
func (e Either[L, R]) Left() Option[L] { return FromTuple(e.left, !e.isRight) }

// Right returns the Right value as an Option.
func (e Either[L, R]) Right() Option[R] { return FromTuple(e.right, e.isRight) }

// Unpack returns both slots and true if the Right slot is the live one.
func (e Either[L, R]) Unpack() (L, R, bool) { return e.left, e.right, e.isRight }

// LeftOr returns the Left value, or fallback.
func (e Either[L, R]) LeftOr(fallback L) L {
	if e.isRight {
		return fallback
	}
	return e.left
}

// RightOr returns the Right value, or fallback.
func (e Either[L, R]) RightOr(fallback R) R {
	if !e.isRight {
		return fallback
	}
	return e.right
}

// RightOrElse returns the Right value, or f applied to the Left value.
func (e Either[L, R]) RightOrElse(f func(L) R) R {
	if !e.isRight {
		return f(e.left)
	}
	return e.right
}

// Map applies f to the Right value. (Haskell: fmap.)
func (e Either[L, R]) Map[R2 any](f func(R) R2) Either[L, R2] {
	if !e.isRight {
		return Left[L, R2](e.left)
	}
	return Right[L](f(e.right))
}

// MapLeft applies f to the Left value. (Haskell Bifunctor: first.)
func (e Either[L, R]) MapLeft[L2 any](f func(L) L2) Either[L2, R] {
	if e.isRight {
		return Right[L2](e.right)
	}
	return Left[L2, R](f(e.left))
}

// BiMap applies onLeft or onRight to the value. (Haskell Bifunctor: bimap.)
func (e Either[L, R]) BiMap[L2, R2 any](onLeft func(L) L2, onRight func(R) R2) Either[L2, R2] {
	if e.isRight {
		return Right[L2](onRight(e.right))
	}
	return Left[L2, R2](onLeft(e.left))
}

// AndThen applies f to the Right value and returns its Either. This is the
// monad bind. (Haskell: >>=.)
func (e Either[L, R]) AndThen[R2 any](f func(R) Either[L, R2]) Either[L, R2] {
	if !e.isRight {
		return Left[L, R2](e.left)
	}
	return f(e.right)
}

// OrElse applies f to the Left value and returns its Either. Use it to
// recover from a Left.
func (e Either[L, R]) OrElse[L2 any](f func(L) Either[L2, R]) Either[L2, R] {
	if e.isRight {
		return Right[L2](e.right)
	}
	return f(e.left)
}

// Match calls onLeft or onRight and returns the result. (Haskell: either.)
func (e Either[L, R]) Match[U any](onLeft func(L) U, onRight func(R) U) U {
	if e.isRight {
		return onRight(e.right)
	}
	return onLeft(e.left)
}

// Swap returns the Either with Left and Right exchanged.
func (e Either[L, R]) Swap() Either[R, L] {
	if e.isRight {
		return Left[R, L](e.right)
	}
	return Right[R](e.left)
}

// String returns "Left(<value>)" or "Right(<value>)".
func (e Either[L, R]) String() string {
	if e.isRight {
		return fmt.Sprintf("Right(%v)", e.right)
	}
	return fmt.Sprintf("Left(%v)", e.left)
}
