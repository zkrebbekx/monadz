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
| `Either.Match` and `Either3.Match` must return an `Either` | the method cannot fold to a plain value; you must call `mo.Fold` | `Either.Match[U]` returns any `U` |
| `mo/result` imports `testing` and `testify` in a non-test file | extra packages in your binary | monadz has no dependencies |
| `mo.Err(nil)` is a failure with a nil error | `r.Error()` returns nil and `IsError()` is true | `Err(nil)` stores `ErrNilError` |

`mo.Fold` already folds an `Either`, a `Result`, or an `Option` to a plain
value. Monadz puts the same fold on each type as a `Match` method. Monadz
has no `Either3`.

## Same name, different meaning

> **Warning:** `OrElse`, `LeftOrElse`, and `RightOrElse` exist in both
> libraries with different meanings. In `mo`, they take a fallback value.
> In monadz, they take a function, as in Rust. Rename each call as the table
> below shows.

## Name map

| `mo` | monadz |
|------|------|
| `mo.Some`, `mo.None` | `monadz.Some`, `monadz.None` |
| `mo.TupleToOption(v, ok)` | `monadz.FromTuple(v, ok)` |
| `mo.PointerToOption(p)` | `monadz.FromPtr(p)` |
| `mo.EmptyableToOption(v)` | `monadz.FromZero(v)` (needs a `comparable` `T`; `mo` uses `reflect` and accepts any `T`) |
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
| `e.MapRight(f)` (`f` returns an `Either`) | `e.AndThen(f)` |
| `e.LeftOrElse(v)` | `e.LeftOr(v)` |
| `e.RightOrElse(v)` | `e.RightOr(v)` |
| `mo.Do(f)` | `monadz.Do(f)` (bugs crash) or `monadz.DoRecover(f)` |
| `mo.NewFuture`, `mo.NewTask` | `monadz.NewTask(...)`, then `Run` or `Start` |
