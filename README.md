# monadz

Monads for Go 1.27: `Option`, `Result`, `Either`, `Validation`, `Patch`,
`Lazy`, `Task`, `State`, `Reader`, and `Writer`.

Monadz matches the `Option` and `Result` APIs of Rust and the core monads of
Haskell. It uses Go 1.27 generic methods, so a pipeline reads left to right
and can change the type at each step:

```go
port := monadz.Lookup(env, "PORT"). // Option[string]
	OkOr(ErrNoPort).               // Result[string]
	Then(strconv.Atoi).            // Result[int]
	Ensure(validPort, ErrBadPort)  // Result[int]
```

```sh
go get github.com/zkrebbekx/monadz
```

Monadz needs Go 1.27 or later with the jsonv2 experiment, which is on by
default in Go 1.27. Do not set `GOEXPERIMENT=nojsonv2`. The module has no
dependencies. The tests use only the standard `testing` package.

## Why monadz

| Goal | What monadz does |
|------|------------------|
| Use the shapes Go code already has | `Result.Then` takes `func(T) (U, error)`. `Option.Then` takes `func(T) (U, bool)`. Functions such as `strconv.Atoi`, `url.Parse`, and `os.LookupEnv` work directly, with no adapter. |
| A bug must crash | `Do` recovers only its own early-exit signal. A nil dereference still crashes, unless a `Try` runs in a deferred function (see [Limits](#limits)). `DoRecover` is the explicit opt-in. |
| Context first | A `Task` receives a `context.Context`. The concurrency functions cancel the work that is no longer necessary, and wait until it stops. `Task.Timeout` is the one exception. |
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
| `Task[T]` | work that takes a context and can fail | `async fn` / `ReaderT Context (ExceptT error IO) a` |
| `State[S, A]` | a step that reads and writes a state | `State s a` |
| `Reader[R, A]` | a computation that reads an environment | `Reader r a` |
| `Writer[W, A]` | a value plus a log | `Writer [w] a` |

## Option

```go
planOf := map[string]string{"acme": "pro"}
seats := map[string]int{"pro": 25}

n := monadz.Lookup(planOf, "acme"). // Option[string]
	AndThen(func(plan string) monadz.Option[int] {
		return monadz.Lookup(seats, plan) // Option[int]
	}).
	Filter(func(n int) bool { return n > 0 }).
	ValueOr(5)
fmt.Println(n) // 25

// Then takes func(T) (U, bool), so os.LookupEnv works directly.
home := monadz.Some("HOME").Then(os.LookupEnv) // Option[string]

// In-place changes, like Rust: Take, Replace, Insert, GetOrInsertWith.
var cache monadz.Option[string]
cfg := cache.GetOrInsertWith(func() string { return "loaded" }) // *string
fmt.Println(*cfg, cache) // loaded Some(loaded)
```

`Option` implements `json.Marshaler`, `json.Unmarshaler`, the JSON v2
`MarshalerTo` and `UnmarshalerFrom`, `sql.Scanner`, `driver.Valuer`,
`slog.LogValuer`, and `fmt.Stringer`. In Go 1.27, `encoding/json` (v1) calls
`MarshalJSONTo` and `UnmarshalJSONFrom` first. `MarshalJSON` and
`UnmarshalJSON` exist for code that calls them directly.

Only `None` is zero, so `json:",omitzero"` drops `None` and keeps `Some(0)`.
Use `omitzero`, not `omitempty` (see [Limits](#limits)).

## Result

```go
func LoadInvoice(ctx context.Context, id string) monadz.Result[Invoice] {
	return monadz.Of(repo.Find(ctx, id)). // Result[Row]
		Wrap("load invoice").           // add context to the error
		Then(decodeRow).                // func(Row) (Invoice, error)
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
inv := monadz.Do(func() Invoice {
	cust := repo.Customer(ctx, id).Try()
	plan := monadz.Lookup(plans, cust.PlanID).OkOr(ErrNoPlan).Try()
	return rate(cust, plan)
})
```

Monadz has two variants. They differ only in how they treat a panic that is
NOT a `Try` signal:

| Function | A failed `Try` | A real panic (a bug) |
|----------|----------------|----------------------|
| `Do` | returns `Err` | **crashes**, as a bug should |
| `DoRecover` | returns `Err` | returns `Err` with a `*PanicError` and its stack |

Use `Do` by default. Use `DoRecover` at a boundary where a crash costs more
than one failed request, for example an HTTP handler or a queue consumer.

Monadz does not use the name `MustDo`. In Go, `Must` means "panic if this
fails" (`regexp.MustCompile`). A `MustDo` that recovers panics would mean the
opposite. `Must()` stays a method with the normal meaning: return the value,
or panic.

`DoOption` is the same idea for `Option`: `Try` on `None` makes it return
`None`.

## Either

Use `Either` when both outcomes are valid and carry data:

```go
func CheckFeature(t Tenant, f Feature) monadz.Either[Paywall, Grant] { ... }

resp := CheckFeature(tenant, SSO).Match(
	func(p Paywall) Response { return upgradeOffer(p) }, // HTTP 402
	func(g Grant) Response { return allowed(g) },
)
```

`Either` is right-biased: `Map`, `AndThen`, `MapOr`, and `Inspect` work on
`Right`. `MapLeft`, `InspectLeft`, `BiMap`, `OrElse`, `LeftOrElse`, and
`Swap` work on `Left`. `LeftOr` and `RightOr` are Haskell's `fromLeft` and
`fromRight`.

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

Each error is a `*monadz.FieldError`, so an API can map it to the field.

`ValidIf(v, ok, err)` makes one check. `ZipWith` combines two `Validation`
values and keeps the errors of both. `Match`, `ValueOr`, `ValueOrElse`,
`ValueOrZero`, `Must`, `Expect`, `Inspect`, and `MapErr` work as they do on
`Result`.

## Patch

A JSON PATCH body has three states per field. An `Option` cannot tell
`null` from "not sent". `Patch` can:

```go
type UpdateTenant struct {
	Name monadz.Patch[string] `json:"name,omitzero"`
	Logo monadz.Patch[string] `json:"logo,omitzero"`
}

// {"logo": null}  ->  Name is Missing (keep), Logo is Null (clear)
req.Name.Apply(&tenant.Name)
req.Logo.ApplyOption(&tenant.Logo)
```

`Patch` works with JSON v1 and v2. `Option()` returns `Some` for a set value
and `None` for the other two states. `Map` changes a set value, and `Match`
handles each of the three states.

## Task

A `Task` describes work. Nothing runs until `Run`. You can run it again, so
retry and timeout are methods:

```go
charge := monadz.NewTask(func(ctx context.Context) (ChargeID, error) {
	return payments.Charge(ctx, req) // same idempotency key on every attempt
}).
	Timeout(5 * time.Second).
	Retry(monadz.RetryPolicy{
		MaxAttempts: 4,
		Backoff:     monadz.ExponentialBackoff(200*time.Millisecond, 5*time.Second),
	})

id := charge.Run(ctx)
```

A nil `RetryIf` retries every error, including the timeout of one attempt.
`Retry` stops when its own ctx ends.

`Wrap`, `Wrapf`, `Ensure`, `MapErr`, and `Recover` work as they do on
`Result`. A task that calls `runtime.Goexit` gives `Err(monadz.ErrGoexit)`.
`Once` caches the first result and a panic, as `sync.OnceValue` does. It
does not cache a context error.

Package `tasks` runs many tasks at once:

| Function | Returns | On a failure |
|----------|---------|--------------|
| `tasks.All(ts...)` | all values, in input order | cancels the rest, returns the failure |
| `tasks.AllLimit(n, ts...)` | same as `All`, at most n at once | same |
| `tasks.Traverse(items, n, f)` | `f` over each item, at most n at once | same |
| `tasks.AllSettled(ts...)` | every `Result` | does not cancel |
| `tasks.Any(ts...)` | the first success | joins all errors if every task fails |
| `tasks.Race(ts...)` | the result of the first task that stops | — |
| `tasks.Zip(a, b)` | both values | cancels the other |

Every function returns only after every task that it started has stopped.
`Task.Timeout` is the one exception to this rule. It returns at the
deadline, and it does not wait for the work, even work that observes ctx.

## State, Reader, Writer

```go
// A pure, replayable rate-limiter step: refill one token, then take one.
type Bucket struct{ Tokens int }

refill := monadz.ModifyState(func(b Bucket) Bucket { b.Tokens++; return b })
take := monadz.NewState(func(b Bucket) (bool, Bucket) {
	if b.Tokens == 0 {
		return false, b
	}
	b.Tokens--
	return true, b
})
allowed, next := refill.And(take).Run(Bucket{})
fmt.Println(allowed, next.Tokens) // true 0

// Dependencies through a Reader.
type Config struct{ Brand string }

greeting := monadz.NewReader(func(c Config) string { return c.Brand }).
	Map(func(b string) string { return "Welcome to " + b })
fmt.Println(greeting.Run(Config{Brand: "Acme"})) // Welcome to Acme

// An audit log through a Writer.
w := monadz.Tell("plan changed").AndThen(func(monadz.Unit) monadz.Writer[string, int] {
	return monadz.NewWriter(25, "seats set")
})
seatCount, entries := w.Run()
fmt.Println(seatCount, entries) // 25 [plan changed seats set]
```

## Helper packages

The plural packages hold functions over many values, in the style of the
standard `slices` and `maps` packages:

| Package | Functions |
|---------|-----------|
| `options` | `Zip`, `Zip3`, `Map3`, `Unzip`, `Flatten`, `Transpose`, `Collect`, `Traverse`, `Values`, `FilterMap`, `First`, `Compose`, `Equal`, `Contains`, `Find`, `Head`, `Last`, `Min`, `Max` |
| `results` | `Zip`, `Zip3`, `Map3`, `Flatten`, `Transpose`, `FromFunc`, `Compose`, `Collect`, `FromSeq2`, `Seq`, `Traverse`, `TraverseFunc`, `Fold`, `Partition`, `Oks`, `Errs`, `First` |
| `eithers` | `Partition`, `Lefts`, `Rights`, `Flatten`, `ToResult`, `Collect` |
| `validations` | `Map2` … `Map6`, `Collect`, `Traverse`, `Errors` |
| `tasks` | `All`, `AllLimit`, `AllSettled`, `Any`, `Race`, `Traverse`, `Zip`, `Zip3` |

The collection functions work on `iter.Seq`, so they connect to
`slices.Values`, `maps.Keys`, and your own iterators. `results.FromSeq2`
collects a `(row, error)` iterator in one call.

## Limits

- Do not call `Try` in a deferred function. During a panic, the `Try` signal
  replaces the panic, so `Do` returns `Err` and the program does not crash.
- Do not call `Try` under code that recovers panics, for example a `String`
  method that `fmt` calls, or a `text/template` function. That code recovers
  the signal, so the failure is lost.
- Use `omitzero`, not `omitempty`. JSON v2 `omitempty` drops every field that
  encodes as `null` or `""`. It drops a Null `Patch` and `Some("")`.
- `Some(None)` and `Some` of a nil pointer encode as `null`, and decode as
  `None`. Rust serde does the same. Use `Patch` when you must keep the
  difference between `null` and a missing field.

## Go 1.27 notes

Go 1.27 generic methods have limits that shape this API. See
[docs/go127.md](docs/go127.md).

1. An interface method cannot have type parameters. Go has no `Monad`
   interface, so each type has its own methods.
2. A method of a generic type cannot mention a type that grows from its own
   type parameter ("instantiation cycle"). This rule dates from Go 1.18 and
   applies to every method. Generic methods only make you meet it more often.
   `Zip` would return `Option[Pair[T, U]]`, so `Zip` is a function in each
   helper package. The `ZipWith` methods are allowed.
3. A receiver cannot be `Option[Option[T]]`, because a receiver type
   parameter must be an identifier. So `Flatten` and `Transpose` are
   functions.

## License

MIT
