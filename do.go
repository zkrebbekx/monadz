package monadz

import "fmt"

// earlyErr is the signal that Result.Try sends to Do.
type earlyErr struct{ err error }

func (e *earlyErr) Error() string {
	return fmt.Sprintf("monadz: Result.Try failed outside monadz.Do: %v", e.err)
}

// earlyNone is the signal that Option.Try sends to Do or DoOption.
type earlyNone struct{}

func (*earlyNone) Error() string {
	return "monadz: Option.Try got None outside monadz.Do or monadz.DoOption"
}

var noneSignal = &earlyNone{}

// Do runs f and returns its value as Ok. Inside f, call Try on a Result or an
// Option to get its value. If the Result is Err or the Option is None, Try
// stops f at once and Do returns the error. This is Go's version of Rust's
// "?" operator and Haskell's do notation:
//
//	monadz.Do(func() Invoice {
//		cust := repo.Customer(ctx, id).Try()
//		plan := monadz.Lookup(plans, cust.PlanID).OkOr(ErrNoPlan).Try()
//		return rate(cust, plan)
//	})
//
// Do recovers ONLY the signal from Try. Any other panic (a nil dereference,
// an index out of range, an explicit panic) passes through, so a bug still
// crashes. Use DoRecover to turn every panic into an error.
//
// An Option.Try on None returns ErrNone. Add context with OkOr first.
//
// Call Try only in the goroutine that runs f. A Try in another goroutine is
// not inside Do, and its panic crashes the program.
//
// Call Try only where no code between Try and Do recovers panics, for
// example a fmt String method or a text/template function. That code
// catches the Try signal, and Do does not see the failure.
//
// Do not call Try in a deferred function. If a panic is in flight, the Try
// signal replaces it, and Do returns Err instead of crashing. In a defer,
// join the error into a named result with errors.Join.
func Do[T any](f func() T) (res Result[T]) {
	defer func() {
		if p := recover(); p != nil {
			switch s := p.(type) {
			case *earlyErr:
				res = Err[T](s.err)
			case *earlyNone:
				res = Err[T](ErrNone)
			default:
				panic(p)
			}
		}
	}()
	return Ok(f())
}

// DoRecover is Do plus a recover for every other panic. A panic that is not
// a Try signal becomes an Err that holds a *PanicError with the stack trace.
//
// Use DoRecover at a boundary where a crash costs more than a failed
// request: an HTTP handler, a queue consumer, or a plugin call.
func DoRecover[T any](f func() T) (res Result[T]) {
	defer func() {
		if p := recover(); p != nil { // Do re-panicked: p is not a Try signal
			res = Err[T](asPanicError(p))
		}
	}()
	return Do(f)
}

// DoOption runs f and returns its value as Some. Inside f, Option.Try on
// None stops f and DoOption returns None.
//
// Result.Try inside DoOption is a programming error, because None cannot
// carry the error. Its signal passes through as a panic. Convert the Result
// first with Result.Ok.
func DoOption[T any](f func() T) (res Option[T]) {
	defer func() {
		if p := recover(); p != nil {
			if _, ok := p.(*earlyNone); ok {
				res = None[T]()
				return
			}
			panic(p)
		}
	}()
	return Some(f())
}
