package monadz_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/zkrebbekx/monadz"
)

// An Option holds a value or nothing. Map, Filter, and ValueOr work on it
// without a check for nil or a second bool.
func ExampleOption() {
	seats := map[string]int{"pro": 25, "free": 0}

	pro := monadz.Lookup(seats, "pro").
		Filter(func(n int) bool { return n > 0 }).
		ValueOr(5)
	free := monadz.Lookup(seats, "free").
		Filter(func(n int) bool { return n > 0 }).
		ValueOr(5)
	missing := monadz.Lookup(seats, "team")

	fmt.Println(pro, free, missing)
	// Output: 25 5 None
}

// Then takes a Go-shaped function func(T) (U, error), such as strconv.Atoi.
// Wrap adds context to the error, and Match folds the Result to one value.
func ExampleResult_Then() {
	parse := func(s string) monadz.Result[int] {
		return monadz.Ok(s).
			Then(strconv.Atoi).
			Wrap("parse port")
	}

	for _, s := range []string{"8080", "http"} {
		msg := parse(s).Match(
			func(port int) string { return "port " + strconv.Itoa(port) },
			func(err error) string { return "error: " + err.Error() },
		)
		fmt.Println(msg)
	}
	// Output:
	// port 8080
	// error: parse port: strconv.Atoi: parsing "http": invalid syntax
}

// Inside Do, Try returns the value of a Result or an Option. On a failure,
// Try stops the block and Do returns the error.
func ExampleDo() {
	errNoPlan := errors.New("no plan")
	plans := map[string]int{"pro": 25}

	seats := func(plan string) monadz.Result[int] {
		return monadz.Do(func() int {
			n := monadz.Lookup(plans, plan).OkOr(errNoPlan).Try()
			return n * 2
		})
	}

	fmt.Println(seats("pro"))
	fmt.Println(seats("team"))
	// Output:
	// Ok(50)
	// Err(no plan)
}

// Do lets a real panic (a bug) crash. DoRecover changes it into an Err that
// holds a *monadz.PanicError with the stack trace.
func ExampleDoRecover() {
	res := monadz.DoRecover(func() int {
		var m map[string]int
		m["x"] = 1 // a bug: assignment to a nil map
		return 0
	})

	var pe *monadz.PanicError
	fmt.Println(res.IsErr(), errors.As(res.Err(), &pe))
	// Output: true true
}

// Use Either when both outcomes are valid and carry data. Match folds both
// sides to one type.
func ExampleEither_Match() {
	check := func(plan string) monadz.Either[string, int] {
		if plan == "free" {
			return monadz.Left[string, int]("upgrade to use SSO")
		}
		return monadz.Right[string](10)
	}

	for _, plan := range []string{"free", "pro"} {
		status := check(plan).Match(
			func(offer string) string { return "402: " + offer },
			func(seats int) string { return "200: " + strconv.Itoa(seats) + " SSO seats" },
		)
		fmt.Println(status)
	}
	// Output:
	// 402: upgrade to use SSO
	// 200: 10 SSO seats
}

// A Patch field tells a missing field from null. Apply keeps the old value
// for a missing field and clears it for null.
func ExamplePatch() {
	type update struct {
		Name monadz.Patch[string] `json:"name,omitzero"`
		Logo monadz.Patch[string] `json:"logo,omitzero"`
	}

	var req update
	if err := json.Unmarshal([]byte(`{"logo": null}`), &req); err != nil {
		panic(err)
	}

	name, logo := "Acme", "acme.png"
	req.Name.Apply(&name)
	req.Logo.Apply(&logo)

	fmt.Printf("%v %v %q %q\n", req.Name, req.Logo, name, logo)
	// Output: Missing Null "Acme" ""
}

// A Task does nothing until Run, so Retry can run it again. A nil RetryIf
// retries every error.
func ExampleTask_Retry() {
	attempts := 0
	flaky := monadz.NewTask(func(ctx context.Context) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("unavailable")
		}
		return "charged", nil
	})

	res := flaky.Retry(monadz.RetryPolicy{MaxAttempts: 4}).Run(context.Background())
	fmt.Println(res, attempts)
	// Output: Ok(charged) 3
}

// AndThen chains State steps. The next step can use the value of the step
// before it.
func ExampleState_AndThen() {
	next := monadz.NewState(func(n int) (int, int) { return n, n + 1 })

	pair := next.AndThen(func(a int) monadz.State[int, string] {
		return next.Map(func(b int) string { return fmt.Sprint(a, "-", b) })
	})

	id, counter := pair.Run(41)
	fmt.Println(id, counter)
	// Output: 41-42 43
}
