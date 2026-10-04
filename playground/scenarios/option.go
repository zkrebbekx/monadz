package scenarios

import (
	"strings"

	"github.com/zkrebbekx/monadz"
)

// TenantID identifies a tenant.
type TenantID string

// Tenant is one customer account of the SaaS.
type Tenant struct {
	ID     TenantID
	Name   string
	Active bool
}

// IsActive reports whether the tenant can sign in.
func (t Tenant) IsActive() bool { return t.Active }

// Branding is the look of a tenant's login page.
type Branding struct {
	Title string `json:"title"`
	Color string `json:"color"`
}

// DefaultBranding is the look for a host with no tenant branding.
var DefaultBranding = Branding{Title: "Sign in", Color: "#64748B"}

var (
	domains = map[string]TenantID{
		"acme.app.example":    "t_acme",
		"globex.app.example":  "t_globex",
		"initech.app.example": "t_initech",
	}
	tenants = map[TenantID]Tenant{
		"t_acme":    {ID: "t_acme", Name: "Acme", Active: true},
		"t_globex":  {ID: "t_globex", Name: "Globex", Active: false},
		"t_initech": {ID: "t_initech", Name: "Initech", Active: true},
	}
	brandings = map[TenantID]Branding{
		"t_acme":   {Title: "Acme sign in", Color: "#00ADD8"},
		"t_globex": {Title: "Globex sign in", Color: "#7C3AED"},
	}
)

// brandingOf returns the branding of a tenant, if it has one.
func brandingOf(t Tenant) (Branding, bool) {
	b, ok := brandings[t.ID]
	return b, ok
}

// normalizeHost lower-cases a host and removes the scheme, "www.", the port,
// and the path. It returns false for an empty host.
func normalizeHost(h string) (string, bool) {
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	if i := strings.IndexAny(h, ":/"); i >= 0 {
		h = h[:i]
	}
	h = strings.TrimPrefix(h, "www.")
	return h, h != ""
}

// OptionInput is the input of the Option scenario.
type OptionInput struct {
	Host string `json:"host"`
}

// DirectoryRow is one row of the fixed tenant directory.
type DirectoryRow struct {
	Host     string `json:"host"`
	TenantID string `json:"tenantId"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Branding string `json:"branding"`
}

// OptionTrace is the result of the Option scenario.
type OptionTrace struct {
	Steps       []Step         `json:"steps"`
	Branding    Branding       `json:"branding"`
	UsedDefault bool           `json:"usedDefault"`
	Directory   []DirectoryRow `json:"directory"`
}

// RunOption finds the login-page branding for a host. Each step can find
// nothing; the first None skips the rest, and ValueOr gives the default.
//
// The playground keeps each intermediate Option in a variable so that it can
// draw each step. The chain is the same as:
//
//	monadz.Some(host).Then(normalizeHost).Then(lookupIn(domains)).
//		Then(lookupIn(tenants)).Filter(Tenant.IsActive).
//		Then(brandingOf).ValueOr(DefaultBranding)
func RunOption(in OptionInput) OptionTrace {
	host := monadz.Some(in.Host).Then(normalizeHost)
	id := host.Then(lookupIn(domains))
	found := id.Then(lookupIn(tenants))
	active := found.Filter(Tenant.IsActive)
	brand := active.Then(brandingOf)

	steps := []Step{
		optStep("normalize host", monadz.Some(in.Host), host, quote, quote, "The host is empty."),
		optStep("look up domain", host, id, quote, func(t TenantID) string { return string(t) }, "No tenant owns this domain."),
		optStep("find active tenant", id, active, func(t TenantID) string { return string(t) },
			func(t Tenant) string { return t.Name + " (active)" }, inactiveNote(found)),
		optStep("load branding", active, brand, func(t Tenant) string { return t.Name },
			func(b Branding) string { return b.Title + " " + b.Color }, "This tenant has no custom branding."),
	}
	return OptionTrace{
		Steps:       steps,
		Branding:    brand.ValueOr(DefaultBranding),
		UsedDefault: brand.IsNone(),
		Directory:   directory(),
	}
}

func inactiveNote(found monadz.Option[Tenant]) string {
	if t, ok := found.Get(); ok && !t.Active {
		return t.Name + " exists, but the account is suspended. Filter turns it into None."
	}
	return "No tenant has this ID."
}

// optStep describes one Option step: skipped if the input was None, else
// Some or None.
func optStep[T, U any](name string, in monadz.Option[T], out monadz.Option[U], showIn func(T) string, showOut func(U) string, noneNote string) Step {
	s := Step{Name: name, Input: "None", Output: "None"}
	if v, ok := in.Get(); ok {
		s.Input = showIn(v)
	} else {
		s.Track = TrackSkipped
		s.Note = "An earlier step gave None, so this step does not run."
		return s
	}
	if v, ok := out.Get(); ok {
		s.Track, s.Output = TrackSome, "Some("+showOut(v)+")"
	} else {
		s.Track, s.Note = TrackNone, noneNote
	}
	return s
}

func directory() []DirectoryRow {
	var rows []DirectoryRow
	for _, host := range []string{"acme.app.example", "globex.app.example", "initech.app.example"} {
		id := domains[host]
		t := tenants[id]
		status := "active"
		if !t.Active {
			status = "suspended"
		}
		brand := monadz.Lookup(brandings, id).Map(func(b Branding) string { return b.Color }).ValueOr("none")
		rows = append(rows, DirectoryRow{Host: host, TenantID: string(id), Name: t.Name, Status: status, Branding: brand})
	}
	return rows
}
