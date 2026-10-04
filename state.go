package monadz

import "slices"

// State is a step that reads a state S and returns a value A and a new
// state. Nothing runs until Run. (Haskell: State s a.)
//
// Build a large computation from small steps with AndThen, then run it
// against any start state. If the steps are pure, the run is a pure
// function, so you can replay it in a test.
//
// The zero State has no step. Run on a zero State panics; use NewState.
type State[S, A any] struct {
	run func(S) (A, S)
}

// NewState wraps a state function.
func NewState[S, A any](f func(S) (A, S)) State[S, A] { return State[S, A]{run: f} }

// StateOf returns a step that leaves the state alone and returns a.
// (Haskell: pure.)
func StateOf[S, A any](a A) State[S, A] {
	return NewState(func(s S) (A, S) { return a, s })
}

// GetState returns a step whose value is the current state. (Haskell: get.)
func GetState[S any]() State[S, S] {
	return NewState(func(s S) (S, S) { return s, s })
}

// PutState returns a step that replaces the state. (Haskell: put.)
func PutState[S any](s S) State[S, Unit] {
	return NewState(func(S) (Unit, S) { return Unit{}, s })
}

// ModifyState returns a step that applies f to the state. (Haskell: modify.)
func ModifyState[S any](f func(S) S) State[S, Unit] {
	return NewState(func(s S) (Unit, S) { return Unit{}, f(s) })
}

// GetStateWith returns a step whose value is f(state). (Haskell: gets.)
func GetStateWith[S, A any](f func(S) A) State[S, A] {
	return NewState(func(s S) (A, S) { return f(s), s })
}

// Run runs the step and returns the value and the final state. Run panics
// on a zero State.
func (m State[S, A]) Run(s S) (A, S) {
	if m.run == nil {
		panic("monadz: Run on a zero State; use NewState")
	}
	return m.run(s)
}

// Eval runs the step and returns only the value. (Haskell: evalState.)
func (m State[S, A]) Eval(s S) A { a, _ := m.Run(s); return a }

// Exec runs the step and returns only the final state. (Haskell: execState.)
func (m State[S, A]) Exec(s S) S { _, next := m.Run(s); return next }

// Map applies f to the value.
func (m State[S, A]) Map[B any](f func(A) B) State[S, B] {
	return NewState(func(s S) (B, S) { a, next := m.Run(s); return f(a), next })
}

// AndThen runs this step, passes its value to f, and runs the step that f
// returns with the new state. This is the monad bind. (Haskell: >>=.)
func (m State[S, A]) AndThen[B any](f func(A) State[S, B]) State[S, B] {
	return NewState(func(s S) (B, S) { a, next := m.Run(s); return f(a).Run(next) })
}

// And runs this step and then next, and keeps the value of next.
// (Haskell: >>.)
func (m State[S, A]) And[B any](next State[S, B]) State[S, B] {
	return NewState(func(s S) (B, S) { _, mid := m.Run(s); return next.Run(mid) })
}

// Reader is a computation that reads a shared environment R, such as a
// config or a set of dependencies. Nothing runs until Run.
// (Haskell: Reader r a.)
//
// The zero Reader has no function. Run on a zero Reader panics; use
// NewReader.
type Reader[R, A any] struct {
	run func(R) A
}

// NewReader wraps a function of the environment. (Haskell: asks.)
func NewReader[R, A any](f func(R) A) Reader[R, A] { return Reader[R, A]{run: f} }

// ReaderOf returns a Reader that ignores the environment and returns a.
func ReaderOf[R, A any](a A) Reader[R, A] {
	return NewReader(func(R) A { return a })
}

// Ask returns a Reader whose value is the environment. (Haskell: ask.)
func Ask[R any]() Reader[R, R] { return NewReader(func(r R) R { return r }) }

// Run runs the Reader with the environment r. Run panics on a zero Reader.
func (m Reader[R, A]) Run(r R) A {
	if m.run == nil {
		panic("monadz: Run on a zero Reader; use NewReader")
	}
	return m.run(r)
}

// Map applies f to the value.
func (m Reader[R, A]) Map[B any](f func(A) B) Reader[R, B] {
	return NewReader(func(r R) B { return f(m.Run(r)) })
}

// AndThen passes the value to f and runs the Reader that f returns with the
// same environment. (Haskell: >>=.)
func (m Reader[R, A]) AndThen[B any](f func(A) Reader[R, B]) Reader[R, B] {
	return NewReader(func(r R) B { return f(m.Run(r)).Run(r) })
}

// Local runs the Reader with a changed environment. (Haskell: local.)
func (m Reader[R, A]) Local(f func(R) R) Reader[R, A] {
	return NewReader(func(r R) A { return m.Run(f(r)) })
}

// Writer is a computation that returns a value A and appends to a log of
// W values, such as audit entries or metrics. (Haskell: Writer w a, with
// w = [W].)
//
// The zero Writer has no value. Run on a zero Writer panics; use NewWriter.
type Writer[W, A any] struct {
	run func() (A, []W)
}

// NewWriter returns a Writer with the value a and the log entries w.
// NewWriter copies w, so a later change to the caller's slice does not
// change the log.
func NewWriter[W, A any](a A, w ...W) Writer[W, A] {
	w = slices.Clone(w)
	return Writer[W, A]{run: func() (A, []W) { return a, w }}
}

// Tell returns a Writer that only appends w to the log. (Haskell: tell.)
func Tell[W any](w ...W) Writer[W, Unit] { return NewWriter(Unit{}, w...) }

// Run returns the value and a copy of the full log. Run panics on a zero
// Writer.
func (m Writer[W, A]) Run() (A, []W) {
	a, w := m.get()
	return a, append([]W(nil), w...) // nil for an empty log
}

// get is Run without the copy. The methods use it to pass the log along.
func (m Writer[W, A]) get() (A, []W) {
	if m.run == nil {
		panic("monadz: Run on a zero Writer; use NewWriter")
	}
	return m.run()
}

// Map applies f to the value. The log does not change.
func (m Writer[W, A]) Map[B any](f func(A) B) Writer[W, B] {
	return Writer[W, B]{run: func() (B, []W) { a, w := m.get(); return f(a), w }}
}

// AndThen passes the value to f, and joins the two logs in order.
// (Haskell: >>=.)
func (m Writer[W, A]) AndThen[B any](f func(A) Writer[W, B]) Writer[W, B] {
	return Writer[W, B]{run: func() (B, []W) {
		a, w1 := m.get()
		b, w2 := f(a).get()
		return b, slices.Concat(w1, w2)
	}}
}

// Censor applies f to the log. f receives a copy, so it can change the
// slice in place. (Haskell: censor.)
func (m Writer[W, A]) Censor(f func([]W) []W) Writer[W, A] {
	return Writer[W, A]{run: func() (A, []W) { a, w := m.get(); return a, f(slices.Clone(w)) }}
}
