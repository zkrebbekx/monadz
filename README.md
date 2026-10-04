# lift

Monads for Go 1.27: `Option`, `Result`, `Either`, `Validation`, `Patch`,
`Lazy`, `Task`, `State`, `Reader`, and `Writer`.

Lift matches the `Option` and `Result` APIs of Rust and the core monads of
Haskell. It uses Go 1.27 generic methods, so a pipeline reads left to right
and can change the type at each step:

```go
port := lift.Lookup(env, "PORT").    // Option[string]
	OkOr(ErrNoPort).                 // Result[string]
	Then(strconv.Atoi).              // Result[int]
	Ensure(validPort, ErrBadPort)    // Result[int]
```

```sh
go get github.com/zkrebbekx/lift
```

Lift needs Go 1.27 or later. It has no dependencies outside the standard
library (goconvey is a test-only dependency).

## Why lift

| Goal | What lift does |
|------|----------------|
| Use the shapes Go code already has | `Result.Then` takes `func(T) (U, error)`. `Option.Then` takes `func(T) (U, bool)`. `strconv.Atoi`, `url.Parse`, and map lookups plug in with no adapter. |
| A bug must crash | `Do` recovers only its own early-exit signal. A nil dereference still crashes. `DoRecover` is the explicit opt-in. |
| Context first | A `Task` receives a `context.Context`. The concurrency functions cancel the work that is no longer needed, and wait for it to stop. |
| Panics keep their stack | A panic in a task that runs in another goroutine is raised again in the waiting goroutine, with the original stack trace. |
| Standard library integration | `iter.Seq`, `encoding/json` v1 **and** v2, `database/sql`, `log/slog`, and `fmt`. |
| No silent wrong answers | `Err(nil)` is still a failure. `omitzero` keeps `Some(0)`. A PATCH field can tell `null` from "not sent". |

See [docs/parity.md](docs/parity.md) for a method-by-method comparison with
Rust and Haskell, and [docs/from-mo.md](docs/from-mo.md) if you use
`samber/mo` today.

## The types

| Type | Holds | Rust / Haskell equivalent |
|------|-------|---------------------------|
| `Option[T]` | a `T`, or nothing | `Option<T>` / `Maybe a` |
| `Result[T]` | a `T`, or an `error` | `Result<T, E>` / `Either e a` |
| `Either[L, R]` | an `L` or an `R`, both valid | `Either a b` |
| `Validation[T]` | a `T`, or ALL the errors | `Data.Validation` |
| `Patch[T]` | not sent, `null`, or a value | — |
| `Lazy[T]` | a `T` computed on first use, then cached | `LazyLock<T>` / lazy evaluation |
| `Task[T]` | work that takes a context and can fail | `async fn` / `IO` + `ExceptT` |
| `State[S, A]` | a step that reads and writes a state | `State s a` |
| `Reader[R, A]` | a computation that reads an environment | `Reader r a` |
| `Writer[W, A]` | a value plus a log | `Writer [w] a` |

## Option

```go
seats := lift.Lookup(planSeats, tenant.Plan). // Option[int]
	Filter(func(n int) bool { return n > 0 }).
	ValueOr(DefaultSeats)

// Map lookups chain with Then, because a lookup returns (V, bool).
limit := lift.Some(tenantID).
	Then(lookupIn(planOf)).   // Option[string]
	Then(lookupIn(limits))    // Option[int]

// In-place changes, like Rust: Take, Replace, Insert, GetOrInsertWith.
cache.GetOrInsertWith(loadConfig)
```

`Option` implements `json.Marshaler`, the JSON v2 `MarshalerTo` and
`UnmarshalerFrom`, `sql.Scanner`, `driver.Valuer`, `slog.LogValuer`, and
`fmt.Stringer`. Only `None` is zero, so `json:",omitzero"` drops `None` and
keeps `Some(0)`.

## Result

```go
func LoadInvoice(ctx context.Context, id string) lift.Result[Invoice] {
	return lift.Of(repo.Find(ctx, id)).        // Result[Row]
		Wrap("load invoice").                  // add context to the error
		Then(decodeRow).                       // func(Row) (Invoice, error)
		Ensure(Invoice.IsOpen, ErrClosed).
		InspectErr(logFailure)
}

status := res.Match(
	func(inv Invoice) int { return http.StatusOK },
	func(err error) int { return statusFor(err) },
)
```

`errors.Is` and `errors.As` work on `res.Err()` through every step.

## Do and DoRecover: Go's `?` operator

Inside `Do`, call `Try()` on a `Result` or an `Option`. On a failure, `Try`
stops the block and `Do` returns the error:

```go
inv := lift.Do(func() Invoice {
	cust := repo.Customer(ctx, id).Try()
	plan := lift.Lookup(plans, cust.PlanID).OkOr(ErrNoPlan).Try()
	return rate(cust, plan)
})
```

Lift has two variants. They differ only in how they treat a panic that is
NOT a `Try` signal:

| Function | A failed `Try` | A real panic (a bug) |
|----------|----------------|----------------------|
| `Do` | returns `Err` | **crashes**, as a bug should |
| `DoRecover` | returns `Err` | returns `Err` with a `*PanicError` and its stack |

Use `Do` by default. Use `DoRecover` at a boundary where a crash costs more
than one failed request, for example an HTTP handler or a queue consumer.

Lift does not use the name `MustDo`. In Go, `Must` means "panic if this
fails" (`regexp.MustCompile`). A `MustDo` that recovers panics would mean the
opposite. `Must()` stays a method with the normal meaning: return the value,
or panic.

`DoOption` is the same idea for `Option`: `Try` on `None` makes it return
`None`.

## Either

Use `Either` when both outcomes are valid and carry data:

```go
func Check(t Tenant, f Feature) lift.Either[Paywall, Grant] { ... }

resp := Check(tenant, SSO).Match(
	func(p Paywall) Response { return upgradeOffer(p) },  // HTTP 402
	func(g Grant) Response { return allowed(g) },
)
```

`Either` is right-biased: `Map` and `AndThen` work on `Right`. `MapLeft`,
`BiMap`, `OrElse`, and `Swap` cover the left side.

## Validation

`Result` stops at the first error. `Validation` collects all of them:

```go
signup := validations.Map3(
	email(req.Email).Field("email"),
	password(req.Password).Field("password"),
	company(req.Company).Field("company"),
	NewSignup,
)
// Invalid(email: not an email; password: too short)
```

Each error is a `*lift.FieldError`, so an API can map it to the field.

## Patch

A JSON PATCH body has three states per field. An `Option` cannot tell
`null` from "not sent". `Patch` can:

```go
type UpdateTenant struct {
	Name lift.Patch[string] `json:"name,omitzero"`
	Logo lift.Patch[string] `json:"logo,omitzero"`
}

// {"logo": null}  ->  Name is Missing (keep), Logo is Null (clear)
req.Name.Apply(&tenant.Name)
req.Logo.ApplyOption(&tenant.Logo)
```

`Patch` works with JSON v1 and v2.

## Task

A `Task` describes work. Nothing runs until `Run`. You can run it again, so
retry and timeout are methods:

```go
charge := lift.NewTask(func(ctx context.Context) (ChargeID, error) {
	return payments.Charge(ctx, req) // same idempotency key on every attempt
}).
	Timeout(5 * time.Second).
	Retry(lift.RetryPolicy{
		MaxAttempts: 4,
		Backoff:     lift.ExponentialBackoff(200*time.Millisecond, 5*time.Second),
		RetryIf:     isTransient,
	})

id := charge.Run(ctx)
```

Package `tasks` runs many tasks at once:

| Function | Returns | On a failure |
|----------|---------|--------------|
| `tasks.All(ts...)` | all values, in input order | cancels the rest, returns the failure |
| `tasks.Limit(n, ts...)` | same as `All`, at most n at once | same |
| `tasks.Traverse(items, n, f)` | `f` over each item, at most n at once | same |
| `tasks.AllSettled(ts...)` | every `Result` | does not cancel |
| `tasks.Any(ts...)` | the first success | joins all errors if every task fails |
| `tasks.Race(ts...)` | the first task to stop | — |
| `tasks.Zip(a, b)` | both values | cancels the other |

Every function returns only after every task that it started has stopped.

## State, Reader, Writer

```go
// A pure, replayable rate limiter step.
allow := refill(now).AndThen(func(lift.Unit) lift.State[Bucket, bool] {
	return take(cost)
})
decision, nextBucket := allow.Run(bucket)

// Dependencies through a Reader.
greeting := lift.Asks(func(c Config) string { return c.Brand }).
	Map(func(b string) string { return "Welcome to " + b })
greeting.Run(cfg)

// An audit log through a Writer.
w := lift.Tell(AuditEntry{"plan changed"}).AndThen(...)
value, entries := w.Run()
```

## Helper packages

The plural packages hold functions over many values, in the style of the
standard `slices` and `maps` packages:

| Package | Functions |
|---------|-----------|
| `options` | `Zip`, `Zip3`, `Map3`, `Unzip`, `Flatten`, `Transpose`, `Collect`, `Traverse`, `Values`, `FilterMap`, `First`, `Compose`, `Equal`, `Contains`, `Find`, `Head`, `Last`, `Min`, `Max` |
| `results` | `Zip`, `Zip3`, `Map3`, `Flatten`, `Transpose`, `Lift`, `Compose`, `Collect`, `FromSeq2`, `Seq`, `Traverse`, `TraverseFunc`, `Fold`, `Partition`, `Oks`, `Errs`, `First` |
| `eithers` | `Partition`, `Lefts`, `Rights`, `Flatten`, `ToResult`, `Collect` |
| `validations` | `Map2` … `Map6`, `Collect`, `Traverse`, `Errors` |
| `tasks` | `All`, `Limit`, `AllSettled`, `Any`, `Race`, `Traverse`, `Zip`, `Zip3` |

They work on `iter.Seq`, so they connect to `slices.Values`, `maps.Keys`,
and your own iterators. `results.FromSeq2` collects a `(row, error)`
iterator in one call.

## Go 1.27 notes

Go 1.27 generic methods have two limits that shape this API. See
[docs/go127.md](docs/go127.md).

1. An interface method cannot have type parameters. Go has no `Monad`
   interface, so each type has its own methods.
2. A method of `Option[T]` cannot mention `Option[Pair[T, U]]`, because the
   type grows from `T` ("instantiation cycle"). So `Zip` is a function in
   each helper package. The `ZipWith` methods are allowed.

## License

MIT
