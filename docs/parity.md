# Parity with Rust and Haskell

This page maps the standard Rust and Haskell operations to monadz. A dash (—)
means that monadz has no equivalent, and the "Note" column says why.

## Rust `Option<T>`

| Rust | monadz | Note |
|------|------|------|
| `Some(v)`, `None` | `monadz.Some(v)`, `monadz.None[T]()` | |
| `is_some`, `is_none` | `IsSome`, `IsNone` | |
| `is_some_and`, `is_none_or` | `IsSomeAnd`, `IsNoneOr` | |
| `expect`, `unwrap` | `Expect`, `Must` | |
| `unwrap_or`, `unwrap_or_else`, `unwrap_or_default` | `ValueOr`, `ValueOrElse`, `ValueOrZero` | |
| `map` | `Map` | can change the type |
| — | `Then` | takes `func(T) (U, bool)`, such as `os.LookupEnv` |
| `inspect` | `Inspect` | |
| `map_or`, `map_or_else` | `MapOr`, `MapOrElse` | |
| `ok_or`, `ok_or_else` | `OkOr`, `OkOrElse` | |
| `and`, `and_then` | `And`, `AndThen` | |
| `filter` | `Filter` | |
| `or`, `or_else`, `xor` | `Or`, `OrElse`, `Xor` | |
| `insert`, `get_or_insert`, `get_or_insert_with` | `Insert`, `GetOrInsert`, `GetOrInsertWith` | pointer receiver |
| `take`, `take_if`, `replace` | `Take`, `TakeIf`, `Replace` | pointer receiver |
| `zip`, `zip_with` | `options.Zip`, `ZipWith` | see [go127.md](go127.md) |
| `unzip` | `options.Unzip` | |
| `flatten` | `options.Flatten` | |
| `transpose` | `options.Transpose` | |
| `iter` | `All` | returns `iter.Seq[T]` |
| `?` operator | `Try` inside `DoOption` or `Do` | |
| `bool::then_some`, `bool::then` | `monadz.When`, `monadz.WhenFunc` | |
| `slice::get` | `monadz.At` | a negative index gives `None` |
| `HashMap::get` | `monadz.Lookup` | |
| `collect::<Option<Vec<_>>>` | `options.Collect` | |
| `Iterator::find`, `next`, `last`, `max`, `min` | `options.Find`, `Head`, `Last`, `Max`, `Min` | |
| `filter_map`, `flatten` (iterator) | `options.FilterMap`, `options.Values` | |
| `as_ref`, `as_mut`, `copied`, `cloned` | — | Go has no borrow types |
| `opt == Some(v)`, `is_some_and(\|x\| x == v)` | `options.Contains` | needs `comparable` |

## Rust `Result<T, E>`

| Rust | monadz | Note |
|------|------|------|
| `Ok(v)`, `Err(e)` | `monadz.Ok(v)`, `monadz.Err[T](e)` | `Err(nil)` is still a failure |
| — | `monadz.Of(v, err)`, `monadz.FromErr(err)` | adapt Go `(T, error)` |
| `is_ok`, `is_err`, `is_ok_and`, `is_err_and` | `IsOk`, `IsErr`, `IsOkAnd`, `IsErrAnd` | |
| `ok`, `err` | `Ok`, `Err` | `Err` returns `error`, Go style |
| `expect`, `unwrap`, `unwrap_err` | `Expect`, `Must`, `MustErr` | |
| `unwrap_or`, `unwrap_or_else`, `unwrap_or_default` | `ValueOr`, `ValueOrElse`, `ValueOrZero` | |
| `map`, `map_err` | `Map`, `MapErr` | |
| — | `Then` | takes `func(T) (U, error)`, such as `strconv.Atoi` or `url.Parse` |
| `map_or`, `map_or_else` | `MapOr`, `MapOrElse` | |
| `inspect`, `inspect_err` | `Inspect`, `InspectErr` | |
| `and`, `and_then` | `And`, `AndThen` | |
| `or`, `or_else` | `Or`, `OrElse`, `Recover` | `Recover` takes the Go shape |
| `flatten`, `transpose` | `results.Flatten`, `results.Transpose` | |
| `iter` | `All` | |
| `?` operator | `Try` inside `Do` | |
| `collect::<Result<Vec<_>, _>>` | `results.Collect`, `results.FromSeq2` | |
| anyhow `context` | `Wrap`, `Wrapf` | keeps `errors.Is` |
| `std::panic::catch_unwind` | `monadz.DoRecover` | `*PanicError` keeps the stack |

## Haskell type classes and functions

| Haskell | monadz | Note |
|---------|------|------|
| `fmap`, `<$>` | `Map` | on every monad type and on `Patch` (not `Future`) |
| `pure`, `return` | `Some`, `Ok`, `Right`, `Valid`, `TaskOk`, `StateOf`, `ReaderOf`, `NewWriter` | |
| `>>=` | `AndThen` | on every monad type (not `Patch`, not `Future`) |
| `>>` | `State.And`, `Option.And`, `Result.And` | |
| `>=>` (Kleisli) | `options.Compose`, `results.Compose` | |
| `join` | `options.Flatten`, `results.Flatten`, `eithers.Flatten` | |
| `liftA2` | `ZipWith` | on `Option`, `Result`, `Validation`, `Lazy`, `Task` |
| `liftA3` | `options.Map3`, `results.Map3`, `validations.Map3` | |
| `sequence`, `sequenceA` | `options.Collect`, `results.Collect`, `eithers.Collect`, `tasks.All` | |
| `traverse`, `mapM` | `options.Traverse`, `results.Traverse`, `validations.Traverse`, `tasks.Traverse` | `tasks.Traverse` is concurrent |
| `foldM` | `results.Fold` | |
| `<\|>`, `asum` | `Or`, `options.First`, `results.First` | |
| `mfilter` | `Filter`, `Ensure` | |
| `maybe`, `either` | `Option.Match`, `Either.Match`, `Result.Match` | |
| `fromMaybe` | `ValueOr` | |
| `catMaybes`, `mapMaybe` | `options.Values`, `options.FilterMap` | |
| `lefts`, `rights`, `partitionEithers` | `eithers.Lefts`, `eithers.Rights`, `eithers.Partition` | |
| `fromLeft`, `fromRight` | `Either.LeftOr`, `Either.RightOr` | both take a default |
| `leftToMaybe`, `rightToMaybe` (package `either`) | `Either.Left`, `Either.Right` | return an `Option` |
| `bimap`, `first` (Bifunctor) | `Either.BiMap`, `Either.MapLeft` | |
| `Data.Validation` | `Validation`, `validations.Map2..Map6` | |
| `validation` (fold) | `Validation.Match` | |
| `State`: `get`, `put`, `modify`, `gets`, `evalState`, `execState` | `GetState`, `PutState`, `ModifyState`, `GetStateWith`, `Eval`, `Exec` | |
| `Reader`: `ask`, `asks`, `local` | `Ask`, `NewReader`, `Local` | |
| `Writer`: `tell`, `censor` | `Tell`, `Censor` | the log is a slice |
| `Control.Concurrent.Async`: `concurrently`, `mapConcurrently` | `tasks.Zip`, `tasks.Traverse` | with cancellation |
| `Control.Concurrent.Async`: `race` | `tasks.Race` | `race` takes 2 actions and returns an `Either`; `tasks.Race` takes N tasks of one type |
| `System.Timeout.timeout` (in `base`) | `Task.Timeout` | |
| `Functor` / `Monad` as a type class | — | Go 1.27 forbids type parameters on interface methods |
| Monad transformers (`ExceptT`, `ReaderT`, …) | — | `Task[T]` is close to `ReaderT Context (ExceptT error IO) T` |
| `Cont`, `STM` | — | not useful in Go |

## Rust futures

| Rust | monadz | Note |
|------|------|------|
| `futures::future::join_all`, `try_join_all` | `tasks.AllSettled`, `tasks.All` | |
| `futures::future::select_ok` | `tasks.Any` | `select_ok` keeps only the last error; `tasks.Any` joins all errors |
| `futures::future::select_all` | `tasks.Race` | both take N futures |
| `tokio::time::timeout` | `Task.Timeout` | |

## Where monadz goes further

| Feature | Rust / Haskell | monadz |
|---------|----------------|------|
| Cancel the losers of a race or a failed `All` | Rust: drop the futures; Haskell async: yes | yes, and wait until they stop (`Timeout` does not wait) |
| A panic in a concurrent task | Rust: the task's `JoinError`; Haskell: rethrown | raised again in the waiting goroutine with the original stack |
| A three-state PATCH field | — | `Patch[T]` |
| JSON, SQL, and structured logging | serde / aeson, separate crates | built in, JSON v1 and v2 |
| Field-level validation errors | libraries | `Validation.Field` and `*FieldError` |
| Retry with backoff, run-once | separate crates and packages | `Task.Retry`, `Task.Once` |
