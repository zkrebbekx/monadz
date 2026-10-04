// Package lift provides monadic types for Go 1.27 and later: Option, Result,
// Either, Validation, Patch, Lazy, Task, State, Reader, and Writer.
//
// Lift uses Go 1.27 generic methods. A method can change the type of the
// value, so a pipeline reads left to right:
//
//	port := lift.Lookup(env, "PORT").   // Option[string]
//		OkOr(ErrNoPort).                // Result[string]
//		Then(strconv.Atoi).             // Result[int]
//		Ensure(validPort, ErrBadPort)   // Result[int]
//
// # Design rules
//
//   - Accept the shapes that Go code already has. Result.Then takes
//     func(T) (U, error), and Option.Then takes func(T) (U, bool). Standard
//     library functions plug in with no adapter.
//   - A bug must crash. Do recovers only its own early-exit signal. Use
//     DoRecover when you want to turn other panics into errors, and you get
//     a *PanicError with the stack trace.
//   - Be context-aware. A Task receives a context.Context, and the
//     combinators in package tasks cancel the work that is no longer needed.
//   - Integrate with the standard library: iter.Seq, encoding/json (v1 and
//     v2), database/sql, log/slog, and fmt.
//
// # Packages
//
// Package lift holds the types and their methods. The plural packages hold
// functions that work on many values, in the style of the slices and maps
// packages:
//
//   - options: Collect, Traverse, Values, FilterMap, First, Flatten, ...
//   - results: Collect, Traverse, Partition, FromSeq2, Fold, Map2, ...
//   - eithers: Partition, Lefts, Rights, ...
//   - validations: Map2 ... Map6, Collect, Traverse (error accumulation)
//   - tasks: All, AllSettled, Any, Race, Traverse (with a concurrency limit)
package lift
