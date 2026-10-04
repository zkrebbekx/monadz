package scenarios

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zkrebbekx/monadz"
)

// ErrNoPlan is the error when a tenant's plan is not in the price list.
var ErrNoPlan = errors.New("plan not in the price list")

// Invoice is a monthly invoice.
type Invoice struct {
	Tenant   string
	Plan     string
	Seats    int
	TotalUSD int
}

type billingTenant struct {
	Name   string
	PlanID string
	Seats  int
}

var priceList = map[string]int{"team": 49, "enterprise": 399}

// DoInput is the input of the Do scenario.
type DoInput struct {
	FailStep2 bool   `json:"failStep2"`
	Bug       bool   `json:"bug"`
	Mode      string `json:"mode"` // "do" or "dorecover"
}

// DoTrace is the result of the Do scenario.
type DoTrace struct {
	Mode string `json:"mode"`
	// Outcome is "ok", "err" (a Try failed), "crash" (a panic escaped Do),
	// or "panic_error" (DoRecover turned a panic into an Err).
	Outcome string `json:"outcome"`
	Value   string `json:"value,omitempty"`
	Error   string `json:"error,omitempty"`
	Stack   string `json:"stack,omitempty"`
	Steps   []Step `json:"steps"`
}

// RunDo builds an invoice inside Do or DoRecover.
func RunDo(in DoInput) DoTrace {
	mode := "do"
	if in.Mode == "dorecover" {
		mode = "dorecover"
	}
	t := DoTrace{Mode: mode}
	var ran []string

	buildInvoice := func() Invoice {
		tenant := loadBillingTenant(in.FailStep2).Try()
		ran = append(ran, "load tenant")

		price := monadz.Lookup(priceList, tenant.PlanID).OkOr(ErrNoPlan).Try()
		ran = append(ran, "look up price")

		var usage map[string]int // a bug: the map is nil
		if in.Bug {
			usage["seats"] = tenant.Seats // panics: assignment to entry in nil map
		}
		ran = append(ran, "count seats")

		return Invoice{Tenant: tenant.Name, Plan: tenant.PlanID, Seats: tenant.Seats, TotalUSD: price * tenant.Seats}
	}

	var res monadz.Result[Invoice]
	if mode == "do" {
		var crash any
		res, crash = playgroundBoundary(func() monadz.Result[Invoice] { return monadz.Do(buildInvoice) })
		if crash != nil {
			t.Outcome = "crash"
			t.Error = fmt.Sprint("panic: ", crash)
			t.Steps = doSteps(ran, "crash")
			return t
		}
	} else {
		res = monadz.DoRecover(buildInvoice)
	}

	inv, err := res.Get()
	var pe *monadz.PanicError
	switch {
	case err == nil:
		t.Outcome = "ok"
		t.Value = fmt.Sprintf("Invoice{%s, %s, %d seats, $%d}", inv.Tenant, inv.Plan, inv.Seats, inv.TotalUSD)
		t.Steps = doSteps(ran, "")
	case errors.As(err, &pe):
		t.Outcome = "panic_error"
		t.Error = err.Error()
		t.Stack = shortStack(pe.Stack)
		t.Steps = doSteps(ran, "crash")
	default:
		t.Outcome = "err"
		t.Error = err.Error()
		t.Steps = doSteps(ran, "err")
	}
	return t
}

func loadBillingTenant(oldPlan bool) monadz.Result[billingTenant] {
	if oldPlan {
		return monadz.Ok(billingTenant{Name: "Acme", PlanID: "legacy-2019", Seats: 12})
	}
	return monadz.Ok(billingTenant{Name: "Acme", PlanID: "team", Seats: 12})
}

// playgroundBoundary stands in for the Go runtime: in a real program, a
// panic that escapes Do crashes the process. The playground catches it here
// only to draw the crash.
func playgroundBoundary[T any](f func() T) (v T, crash any) {
	defer func() { crash = recover() }()
	return f(), nil
}

var doStepNames = []string{"load tenant", "look up price", "count seats", "build invoice"}

// doSteps marks the steps that ran as ok, the next step with the stop kind,
// and the rest as skipped.
func doSteps(ran []string, stop string) []Step {
	steps := make([]Step, len(doStepNames))
	for i, name := range doStepNames {
		steps[i] = Step{Name: name, Track: TrackSkipped}
		switch {
		case i < len(ran):
			steps[i].Track = TrackOk
		case i == len(ran) && stop == "":
			steps[i].Track = TrackOk
		case i == len(ran) && stop == "err":
			steps[i].Track, steps[i].Note = TrackErr, "Try got an Err and stopped the block."
		case i == len(ran) && stop == "crash":
			steps[i].Track, steps[i].Note = TrackPanic, "A bug panicked here."
		}
	}
	return steps
}

// shortStack keeps the first lines of a stack trace.
func shortStack(stack []byte) string {
	lines := strings.Split(strings.TrimSpace(string(stack)), "\n")
	if len(lines) > 9 {
		lines = append(lines[:9], "\t…")
	}
	return strings.Join(lines, "\n")
}
