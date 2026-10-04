# Moving from samber/mo

Monadz fixes the sharp edges of `samber/mo` v1.17.0. Each item below was
checked against the `mo` source.

| `mo` behaviour | Effect | monadz |
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
| `mo/result` imports `testing` and `testify` in a non-test file | extra packages in your binary | monadz has no runtime dependencies |
| `mo.Err(nil)` is a failure with a nil error | `Err()` returns nil for a failure | `Err(nil)` stores `ErrNilError` |

## Name map

| `mo` | monadz |
|------|------|
| `mo.Some`, `mo.None` | `monadz.Some`, `monadz.None` |
| `mo.TupleToOption(v, ok)` | `monadz.FromTuple(v, ok)` |
| `mo.PointerToOption(p)` | `monadz.FromPtr(p)` |
| `mo.EmptyableToOption(v)` | `monadz.FromZero(v)` |
| `o.OrElse(v)` | `o.ValueOr(v)` |
| `o.OrEmpty()` | `o.ValueOrZero()` |
| `o.MustGet()` | `o.Must()` |
| `o.ToPointer()` | `o.Ptr()` |
| `mo.Ok`, `mo.Err`, `mo.Errf` | `monadz.Ok`, `monadz.Err`, `monadz.Errf` |
| `mo.TupleToResult(v, err)` | `monadz.Of(v, err)` |
| `mo.Try(f)` | `monadz.Of(f())` |
| `r.Error()` | `r.Err()` |
| `r.OrElse(v)` | `r.ValueOr(v)` |
| `r.ToEither()` | `r.Either()` |
| `result.Map(f)(r)` | `r.Map(f)` |
| `result.FlatMap(f)(r)` | `r.AndThen(f)` |
| `mo.Fold(r, onOk, onErr)` | `r.Match(onOk, onErr)` |
| `mo.Left`, `mo.Right` | `monadz.Left`, `monadz.Right` |
| `e.MapRight(f)` | `e.Map(f)` |
| `mo.Do(f)` | `monadz.Do(f)` (bugs crash) or `monadz.DoRecover(f)` |
| `mo.NewFuture`, `mo.NewTask` | `monadz.NewTask(...)`, then `Run` or `Start` |
