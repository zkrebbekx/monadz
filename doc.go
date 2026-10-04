// Package monadz provides monadic types for Go 1.27 and later: Option, Result,
// Either, Validation, Patch, Lazy, Task, State, Reader, and Writer.
//
// Monadz needs Go 1.27 or later with the jsonv2 experiment, which is on by
// default in Go 1.27. Do not set GOEXPERIMENT=nojsonv2.
//
// Monadz uses Go 1.27 generic methods. A method can change the type of the
// value, so a pipeline reads left to right:
//
//	port := monadz.Lookup(env, "PORT"). // Option[string]
//		OkOr(ErrNoPort).               // Result[string]
//		Then(strconv.Atoi).            // Result[int]
//		Ensure(validPort, ErrBadPort)  // Result[int]
//
// Inside Do, Try gives the value of a Result or an Option, or stops the block
// and makes Do return the error:
//
//	inv := monadz.Do(func() Invoice {
//		cust := repo.Customer(ctx, id).Try()
//		plan := monadz.Lookup(plans, cust.PlanID).OkOr(ErrNoPlan).Try()
//		return rate(cust, plan)
//	})
//
// # Design rules
//
//   - Accept the shapes that Go code already has. Result.Then takes
//     func(T) (U, error), and Option.Then takes func(T) (U, bool). Functions
//     such as strconv.Atoi, url.Parse, and os.LookupEnv work directly.
//   - A bug must crash. Do recovers only its own early-exit signal. Use
//     DoRecover when you want to turn other panics into errors, and you get
//     a *PanicError with the stack trace.
//   - Be context-aware. A Task receives a context.Context, and the
//     functions in package tasks cancel the work that is no longer
//     necessary.
//   - Integrate with the standard library: iter.Seq, encoding/json (v1 and
//     v2), database/sql, log/slog, and fmt.
//
// # Limits
//
//   - Do not call Try in a deferred function. During a panic, the Try signal
//     replaces the panic, so Do returns Err and the program does not crash.
//   - Do not call Try under code that recovers panics, such as a String
//     method that fmt calls, or a text/template function. The failure is lost.
//   - Tag Option and Patch fields with omitzero, not omitempty. JSON v2
//     omitempty drops a Null Patch and Some("").
//   - Some(None) and Some of a nil pointer encode as null and decode as None.
//     Use Patch to tell null from a missing field.
//
// # Packages
//
// Package monadz holds the types and their methods. The plural packages hold
// functions that work on many values, in the style of the slices and maps
// packages:
//
//   - options: Collect, Traverse, Values, FilterMap, First, Flatten, Zip, ...
//   - results: Collect, Traverse, Partition, FromSeq2, Fold, Map3, Zip, ...
//   - eithers: Partition, Lefts, Rights, Collect, ...
//   - validations: Map2 ... Map6, Collect, Traverse (error accumulation)
//   - tasks: All, AllLimit, AllSettled, Any, Race, Traverse (with a
//     concurrency limit)
package monadz
