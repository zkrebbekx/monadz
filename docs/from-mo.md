# Moving from samber/mo

Lift fixes the sharp edges of `samber/mo` v1.17.0. Each item below was
checked against the `mo` source.

| `mo` behaviour | Effect | lift |
|----------------|--------|------|
| `Result[T].Map` must return the same `T` (Go 1.18 limit) | type changes need free functions in `mo/result` and nested `Pipe` calls | `Map`, `Then`, and `AndThen` are generic methods |
| `result.Match` gives `onError` no error | you cannot log or branch on the error | `Result.Match` passes the error |
| `mo.Do` recovers every panic | a nil dereference becomes a quiet `Err` | `Do` re-panics bugs; `DoRecover` is the opt-in |
| `Option.IsZero` is true for `Some(0)` | `omitzero` drops `Some(0)` | only `None` is zero |
| JSON `null` and a missing field both give `None` | PATCH cannot clear a field | `Patch[T]` |
| `Result` has no error accumulation | a form reports one error at a time | `Validation` and `validations.MapN` |
| `State` has no bind | you cannot chain dependent steps | `State.AndThen` |
| `Future` starts at once, has no context, and `Collect` blocks forever if a callback forgets to resolve | leaks and deadlocks | `Task` is lazy and context-first; there is no resolve or reject API |
| `Either3.Match` must return an `Either3` | no fold to a plain value | `Either.Match[U]` returns any `U` |
| `mo/result` imports `testing` and `testify` in a non-test file | extra packages in your binary | lift has no runtime dependencies |
| `mo.Err(nil)` is a failure with a nil error | `Err()` returns nil for a failure | `Err(nil)` stores `ErrNilError` |

## Name map

| `mo` | lift |
|------|------|
| `mo.Some`, `mo.None` | `lift.Some`, `lift.None` |
| `mo.TupleToOption(v, ok)` | `lift.FromTuple(v, ok)` |
| `mo.PointerToOption(p)` | `lift.FromPtr(p)` |
| `mo.EmptyableToOption(v)` | `lift.FromZero(v)` |
| `o.OrElse(v)` | `o.ValueOr(v)` |
| `o.OrEmpty()` | `o.ValueOrZero()` |
| `o.MustGet()` | `o.Must()` |
| `o.ToPointer()` | `o.Ptr()` |
| `mo.Ok`, `mo.Err`, `mo.Errf` | `lift.Ok`, `lift.Err`, `lift.Errf` |
| `mo.TupleToResult(v, err)` | `lift.Of(v, err)` |
| `mo.Try(f)` | `lift.Of(f())` |
| `r.Error()` | `r.Err()` |
| `r.OrElse(v)` | `r.ValueOr(v)` |
| `r.ToEither()` | `r.Either()` |
| `result.Map(f)(r)` | `r.Map(f)` |
| `result.FlatMap(f)(r)` | `r.AndThen(f)` |
| `mo.Fold(r, onOk, onErr)` | `r.Match(onOk, onErr)` |
| `mo.Left`, `mo.Right` | `lift.Left`, `lift.Right` |
| `e.MapRight(f)` | `e.Map(f)` |
| `mo.Do(f)` | `lift.Do(f)` (bugs crash) or `lift.DoRecover(f)` |
| `mo.NewFuture`, `mo.NewTask` | `lift.NewTask(...)`, then `Run` or `Start` |
