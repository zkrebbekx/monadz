package monadz

import "fmt"

// Unit is the type with one value. Use it where a type parameter needs a
// type, but the value carries no data.
type Unit struct{}

// Pair holds two values. The Zip functions in the plural packages return a Pair.
type Pair[A, B any] struct {
	First  A
	Second B
}

// PairOf returns a Pair.
func PairOf[A, B any](a A, b B) Pair[A, B] { return Pair[A, B]{a, b} }

// Unpack returns the two values.
func (p Pair[A, B]) Unpack() (A, B) { return p.First, p.Second }

// Swap returns the Pair with the values in the other order.
func (p Pair[A, B]) Swap() Pair[B, A] { return Pair[B, A]{p.Second, p.First} }

// String formats the Pair as "(a, b)".
func (p Pair[A, B]) String() string { return fmt.Sprintf("(%v, %v)", p.First, p.Second) }

// Triple holds three values.
type Triple[A, B, C any] struct {
	First  A
	Second B
	Third  C
}

// TripleOf returns a Triple.
func TripleOf[A, B, C any](a A, b B, c C) Triple[A, B, C] { return Triple[A, B, C]{a, b, c} }

// Unpack returns the three values.
func (t Triple[A, B, C]) Unpack() (A, B, C) { return t.First, t.Second, t.Third }

// String formats the Triple as "(a, b, c)".
func (t Triple[A, B, C]) String() string {
	return fmt.Sprintf("(%v, %v, %v)", t.First, t.Second, t.Third)
}
