package scenarios

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/zkrebbekx/monadz"
)

// Plan is a price plan.
type Plan struct {
	Name     string
	MaxSeats int
	Features []string
	PriceUSD int
}

var plans = []Plan{
	{Name: "free", MaxSeats: 3, Features: []string{"api_access"}, PriceUSD: 0},
	{Name: "team", MaxSeats: 25, Features: []string{"api_access", "audit_log"}, PriceUSD: 49},
	{Name: "enterprise", MaxSeats: 500, Features: []string{"api_access", "audit_log", "sso"}, PriceUSD: 399},
}

func planNamed(name string) Plan {
	for _, p := range plans {
		if p.Name == name {
			return p
		}
	}
	return plans[0]
}

// Subscription is the tenant's plan and seat use.
type Subscription struct {
	Plan      Plan
	SeatsUsed int
}

// Grant says that the tenant can use the feature.
type Grant struct {
	Feature string `json:"feature"`
	Plan    string `json:"plan"`
}

// Paywall says that the tenant must upgrade first. It is a normal business
// answer, not a failure.
type Paywall struct {
	Feature   string `json:"feature"`
	Reason    string `json:"reason"`
	UpgradeTo string `json:"upgradeTo"`
	PriceUSD  int    `json:"priceUsd"`
}

// upgradeFor returns the cheapest plan that has the feature and the seats.
func upgradeFor(feature string, seats int) (string, int) {
	for _, p := range plans {
		if slices.Contains(p.Features, feature) && seats <= p.MaxSeats {
			return p.Name, p.PriceUSD
		}
	}
	return "contact sales", 0
}

func requireFeature(feature string) func(Subscription) monadz.Either[Paywall, Subscription] {
	return func(s Subscription) monadz.Either[Paywall, Subscription] {
		if slices.Contains(s.Plan.Features, feature) {
			return monadz.Right[Paywall](s)
		}
		to, price := upgradeFor(feature, s.SeatsUsed)
		return monadz.Left[Paywall, Subscription](Paywall{Feature: feature, Reason: "The " + s.Plan.Name + " plan does not include " + feature + ".", UpgradeTo: to, PriceUSD: price})
	}
}

func requireSeats(feature string) func(Subscription) monadz.Either[Paywall, Subscription] {
	return func(s Subscription) monadz.Either[Paywall, Subscription] {
		if s.SeatsUsed <= s.Plan.MaxSeats {
			return monadz.Right[Paywall](s)
		}
		to, price := upgradeFor(feature, s.SeatsUsed)
		return monadz.Left[Paywall, Subscription](Paywall{Feature: feature, Reason: fmt.Sprintf("%d seats are in use. The %s plan allows %d.", s.SeatsUsed, s.Plan.Name, s.Plan.MaxSeats), UpgradeTo: to, PriceUSD: price})
	}
}

// EitherInput is the input of the Either scenario.
type EitherInput struct {
	Plan    string `json:"plan"`
	Feature string `json:"feature"`
	Seats   int    `json:"seats"`
}

// EitherTrace is the result of the Either scenario.
type EitherTrace struct {
	Steps   []Step   `json:"steps"`
	Side    string   `json:"side"`
	Grant   *Grant   `json:"grant,omitempty"`
	Paywall *Paywall `json:"paywall,omitempty"`
	Status  int      `json:"status"`
	Body    string   `json:"body"`
}

// RunEither checks if a tenant can use a feature. The answer is Right(Grant)
// or Left(Paywall). Both are valid answers.
//
// The chain is:
//
//	monadz.Right[Paywall](sub).AndThen(requireFeature(f)).
//		AndThen(requireSeats(f)).Map(toGrant)
func RunEither(in EitherInput) EitherTrace {
	sub := Subscription{Plan: planNamed(in.Plan), SeatsUsed: clamp(in.Seats, 1, 1000)}
	feature := in.Feature
	if feature == "" {
		feature = "sso"
	}

	start := monadz.Right[Paywall](sub)
	hasFeature := start.AndThen(requireFeature(feature))
	hasSeats := hasFeature.AndThen(requireSeats(feature))
	access := hasSeats.Map(func(s Subscription) Grant {
		return Grant{Feature: feature, Plan: s.Plan.Name}
	})

	showSub := func(s Subscription) string { return s.Plan.Name + ", " + strconv.Itoa(s.SeatsUsed) + " seats" }
	steps := []Step{
		eitherStep("plan includes "+feature, start, hasFeature, showSub, showSub),
		eitherStep("seats fit the plan", hasFeature, hasSeats, showSub, showSub),
		eitherStep("grant access", hasSeats, access, showSub, func(g Grant) string { return "Grant{" + g.Feature + "}" }),
	}

	return access.Match(
		func(p Paywall) EitherTrace {
			body := fmt.Sprintf(`{"upgrade_to":%q,"price_usd":%d,"reason":%q}`, p.UpgradeTo, p.PriceUSD, p.Reason)
			return EitherTrace{Steps: steps, Side: TrackLeft, Paywall: &p, Status: http.StatusPaymentRequired, Body: body}
		},
		func(g Grant) EitherTrace {
			body := fmt.Sprintf(`{"feature":%q,"enabled":true}`, g.Feature)
			return EitherTrace{Steps: steps, Side: TrackRight, Grant: &g, Status: http.StatusOK, Body: body}
		},
	)
}

func eitherStep[A, B any](name string, in monadz.Either[Paywall, A], out monadz.Either[Paywall, B], showIn func(A) string, showOut func(B) string) Step {
	s := Step{Name: name}
	a, ok := in.Right().Get()
	if !ok {
		s.Input, s.Output, s.Track = "Left", "Left", TrackSkipped
		s.Note = "The answer is already Left, so this step does not run."
		return s
	}
	s.Input = showIn(a)
	if b, ok := out.Right().Get(); ok {
		s.Track, s.Output = TrackRight, "Right("+showOut(b)+")"
	} else {
		p := out.LeftOr(Paywall{})
		s.Track, s.Output, s.Note = TrackLeft, "Left(Paywall{"+p.UpgradeTo+"})", p.Reason
	}
	return s
}
