package monadz

import (
	"errors"
	"fmt"
	"runtime/debug"
)

// ErrNone is the error for a missing value: Option.Must on None, and
// Option.Try on None inside Do.
var ErrNone = errors.New("monadz: value is None")

// ErrNilError is the error that Err, Invalid, and similar constructors store
// when you give them a nil error. A nil error cannot mean failure, so monadz
// replaces it with this error instead of returning a success by mistake.
var ErrNilError = errors.New("monadz: failure constructed with a nil error")

// PanicError is a panic that DoRecover or a Task turned into a value.
type PanicError struct {
	// Value is the value that was passed to panic.
	Value any
	// Stack is the stack trace of the goroutine at the time of the panic.
	Stack []byte

	rethrown bool
}

func newPanicError(v any) *PanicError {
	return &PanicError{Value: v, Stack: debug.Stack()}
}

// Error returns "panic: <value>".
func (p *PanicError) Error() string {
	if p.rethrown {
		// The panic crossed goroutines. Put the original stack in the message,
		// because the crash report shows only the stack of the waiting goroutine.
		return fmt.Sprintf("panic: %v\n\noriginal goroutine stack:\n%s", p.Value, p.Stack)
	}
	return fmt.Sprintf("panic: %v", p.Value)
}

// Unwrap returns the panic value if it is an error (for example a
// runtime.Error), so errors.Is and errors.As see it.
func (p *PanicError) Unwrap() error {
	if err, ok := p.Value.(error); ok {
		return err
	}
	return nil
}

func nonNil(err error) error {
	if err == nil {
		return ErrNilError
	}
	return err
}
