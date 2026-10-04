// The Go code that each scenario shows: the monadz version, and the same
// logic in plain Go. Each function gets the current inputs, so the code
// can follow the selected mode.

export const snippets = {
  option: () => ({
    note: "Option.Then accepts the Go shape func(T) (U, bool), so a map lookup plugs in with no adapter.",
    monadz: `func loginBranding(host string) Branding {
	return monadz.Some(host).
		Then(normalizeHost).     // func(string) (string, bool)
		Then(lookupIn(domains)). // Option[TenantID]
		Then(lookupIn(tenants)). // Option[Tenant]
		Filter(Tenant.IsActive). // a suspended tenant becomes None
		Then(brandingOf).        // Option[Branding]
		ValueOr(DefaultBranding) // one default, at the end
}

// lookupIn turns a map into func(K) (V, bool).
func lookupIn[K comparable, V any](m map[K]V) func(K) (V, bool) {
	return func(k K) (V, bool) { v, ok := m[k]; return v, ok }
}`,
    plain: `func loginBranding(host string) Branding {
	h, ok := normalizeHost(host)
	if !ok {
		return DefaultBranding
	}
	id, ok := domains[h]
	if !ok {
		return DefaultBranding
	}
	t, ok := tenants[id]
	if !ok || !t.IsActive() {
		return DefaultBranding
	}
	b, ok := brandingOf(t)
	if !ok {
		return DefaultBranding
	}
	return b
}`,
  }),

  result: () => ({
    note: "Result.Then accepts the Go shape func(T) (U, error). The steps are normal Go functions; the chain moves the value or the error.",
    monadz: `func (h *Handler) Signup(form SignupForm) Response {
	account := monadz.Ok(form).
		Then(validateSignup).      // func(SignupForm) (SignupForm, error)
		Then(h.db.checkEmailFree). // func(SignupForm) (SignupForm, error)
		Map(hashPassword).         // func(SignupForm) NewAccount
		Then(h.db.insert)          // func(NewAccount) (Account, error)

	return account.Match(
		func(a Account) Response { return created(a) },   // 201
		func(err error) Response { return failure(err) }, // 422, 409, or 503
	)
}

// failure maps the error to 422, 409, or 503 with errors.Is.
// The plain Go version uses the same function.`,
    plain: `func (h *Handler) Signup(form SignupForm) Response {
	form, err := validateSignup(form)
	if err != nil {
		return failure(err)
	}
	form, err = h.db.checkEmailFree(form)
	if err != nil {
		return failure(err)
	}
	acct := hashPassword(form)
	a, err := h.db.insert(acct)
	if err != nil {
		return failure(err)
	}
	return created(a)
}`,
  }),

  validation: () => ({
    note: "validations.Map3 runs all three checks, even when the first one fails. Field wraps each error in a *monadz.FieldError.",
    monadz: `func validatePassword(s string) monadz.Validation[string] {
	return validations.Map2( // both checks run
		monadz.ValidIf(s, len(s) >= 8, ErrTooShort),
		monadz.ValidIf(s, hasDigit(s), ErrNoDigit),
		func(p, _ string) string { return p },
	)
}

signup := validations.Map3(
	validateEmail(req.Email).Field("email"),
	validatePassword(req.Password).Field("password"),
	validateCompany(req.Company).Field("company"),
	NewSignupForm, // runs only if all three are Valid
)

for _, err := range signup.Errors() {
	var fe *monadz.FieldError
	if errors.As(err, &fe) {
		resp.Fields[fe.Field] = append(resp.Fields[fe.Field], fe.Err.Error())
	}
}`,
    plain: `var errs []error
if !isEmail(req.Email) {
	errs = append(errs, &FieldError{"email", ErrNotEmail})
}
if len(req.Password) < 8 {
	errs = append(errs, &FieldError{"password", ErrTooShort})
}
if !hasDigit(req.Password) {
	errs = append(errs, &FieldError{"password", ErrNoDigit})
}
company := strings.TrimSpace(req.Company)
if company == "" {
	errs = append(errs, &FieldError{"company", ErrRequired})
} else if len(company) > 40 {
	errs = append(errs, &FieldError{"company", ErrTooLong})
}
if len(errs) > 0 {
	return Response{Status: 422, Errors: errs}
}
form := NewSignupForm(req.Email, req.Password, company)`,
  }),

  either: () => ({
    note: "Either is right-biased: AndThen and Map work on Right. Match makes the caller handle both answers.",
    monadz: `func Check(sub Subscription, feature string) monadz.Either[Paywall, Grant] {
	return monadz.Right[Paywall](sub).
		AndThen(requireFeature(feature)). // Left(Paywall) if the plan lacks it
		AndThen(requireSeats(feature)).   // Left(Paywall) if too many seats
		Map(func(s Subscription) Grant { return Grant{feature, s.Plan.Name} })
}

resp := Check(sub, "sso").Match(
	func(p Paywall) Response { return upgradeOffer(p) }, // 402
	func(g Grant) Response { return enabled(g) },        // 200
)`,
    plain: `func Check(sub Subscription, feature string) (Grant, *Paywall) {
	if !slices.Contains(sub.Plan.Features, feature) {
		return Grant{}, &Paywall{ /* upgrade offer */ }
	}
	if sub.SeatsUsed > sub.Plan.MaxSeats {
		return Grant{}, &Paywall{ /* upgrade offer */ }
	}
	return Grant{feature, sub.Plan.Name}, nil
}

g, paywall := Check(sub, "sso")
if paywall != nil { // the compiler does not make you check this
	return upgradeOffer(*paywall)
}
return enabled(g)`,
  }),

  patch: () => ({
    note: "Patch decodes a missing field as Missing and null as Null. Tag the fields with omitzero so that Missing is not written back.",
    monadz: `type UpdateTenant struct {
	Name         monadz.Patch[string] \`json:"name,omitzero"\`
	Logo         monadz.Patch[string] \`json:"logo,omitzero"\`
	SupportEmail monadz.Patch[string] \`json:"support_email,omitzero"\`
}

var req UpdateTenant
if err := json.Unmarshal(body, &req); err != nil {
	return badRequest(err) // 400
}
if req.Name.IsNull() {
	return unprocessable(ErrNameRequired) // 422: a tenant needs a name
}
req.Name.Apply(&tenant.Name)                       // string
req.Logo.ApplyOption(&tenant.Logo)                 // Option[string]
req.SupportEmail.ApplyOption(&tenant.SupportEmail) // Option[string]`,
    plain: `// Go has no three-state field. One way: decode into raw JSON first.
var raw map[string]json.RawMessage
if err := json.Unmarshal(body, &raw); err != nil {
	return badRequest(err)
}
if v, sent := raw["logo"]; sent {
	if string(v) == "null" {
		tenant.Logo = nil // clear
	} else {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return badRequest(err)
		}
		tenant.Logo = &s
	}
}
// ... the same lines again for "name" (where null is an error)
// ... and again for "support_email"`,
  }),

  do: (s) => {
    const fn = s.mode === "dorecover" ? "DoRecover" : "Do";
    return {
      note: fn === "Do"
        ? "Do recovers only the stop signal from Try. A real panic passes through, so a bug crashes the program."
        : "DoRecover also recovers a real panic. The panic becomes an Err that holds a *monadz.PanicError with the stack trace.",
      monadz: `inv := monadz.${fn}(func() Invoice {
	tenant := loadTenant(id).Try() // Result[Tenant] -> Tenant, or stop
	price := monadz.Lookup(priceList, tenant.PlanID).
		OkOr(ErrNoPlan).
		Try() // stop here if the plan is not in the price list

	var usage map[string]int      // a bug: the map is nil
	usage["seats"] = tenant.Seats // panic: assignment to entry in nil map

	return Invoice{tenant.Name, tenant.PlanID, tenant.Seats, price * tenant.Seats}
})
// inv is a monadz.Result[Invoice]`,
      plain: `func buildInvoice(id string) (inv Invoice, err error) {${fn === "DoRecover" ? `
	defer func() { // turn a panic into an error
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v\\n%s", p, debug.Stack())
		}
	}()` : ""}
	tenant, err := loadTenant(id)
	if err != nil {
		return Invoice{}, err
	}
	price, ok := priceList[tenant.PlanID]
	if !ok {
		return Invoice{}, ErrNoPlan
	}

	var usage map[string]int      // a bug: the map is nil
	usage["seats"] = tenant.Seats // panic: assignment to entry in nil map

	return Invoice{tenant.Name, tenant.PlanID, tenant.Seats, price * tenant.Seats}, nil
}`,
    };
  },

  tasks: (s) => {
    const fn = { all: "All", any: "Any", race: "Race", allSettled: "AllSettled" }[s.mode] || "All";
    const ret = { All: "Result[[]string]", Any: "Result[string]", Race: "Result[string]", AllSettled: "Result[[]Result[string]]" }[fn];
    return {
      note: "Each function in package tasks returns only after every task that it started has stopped. A canceled task sees ctx.Done().",
      monadz: `fetch := func(name string, d time.Duration, fails bool) monadz.Task[string] {
	return monadz.NewTask(func(ctx context.Context) (string, error) {
		if err := monadz.Sleep(ctx, d); err != nil {
			return "", err // canceled: ctx is done
		}
		if fails {
			return "", fmt.Errorf("%s: %w", name, ErrUpstream)
		}
		return name + " data", nil
	})
}

res := tasks.${fn}(usage, invoices, taxRates).Run(ctx) // ${ret}`,
      plain: plainTasks[fn],
    };
  },

  retry: (s) => ({
    note: "A Task describes work and does not run until Run. So Retry can run it again, with the same inputs, after each failure.",
    monadz: `charge := monadz.NewTask(chargeCard).Retry(monadz.RetryPolicy{
	MaxAttempts: ${s.maxAttempts},
	Backoff:     monadz.ExponentialBackoff(${s.baseMs}*time.Millisecond, 2*time.Second),
})

id := charge.Run(ctx) // Result[string]
// Err: "monadz: gave up after ${s.maxAttempts} attempt(s): payment gateway timeout"`,
    plain: `var id string
var err error
wait := ${s.baseMs} * time.Millisecond
for attempt := 1; ; attempt++ {
	id, err = chargeCard(ctx)
	if err == nil {
		break
	}
	if attempt == ${s.maxAttempts} {
		err = fmt.Errorf("gave up after %d attempts: %w", attempt, err)
		break
	}
	t := time.NewTimer(wait)
	select {
	case <-t.C:
	case <-ctx.Done():
		t.Stop()
		return "", ctx.Err()
	}
	wait = min(wait*2, 2*time.Second)
}`,
  }),
};

const plainTasks = {
  All: `ctx, cancel := context.WithCancel(ctx)
defer cancel()
fns := []func(context.Context) (string, error){usage, invoices, taxRates}
values := make([]string, len(fns))
errs := make([]error, len(fns))
var wg sync.WaitGroup
for i, f := range fns {
	wg.Add(1)
	go func() {
		defer wg.Done()
		values[i], errs[i] = f(ctx)
		if errs[i] != nil {
			cancel() // stop the others
		}
	}()
}
wg.Wait() // a panic in a goroutine crashes the program
for _, err := range errs {
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil, err
	}
}
return values, nil`,
  Any: `ctx, cancel := context.WithCancel(ctx)
defer cancel()
fns := []func(context.Context) (string, error){usage, invoices, taxRates}
type out struct {
	v   string
	err error
}
ch := make(chan out, len(fns))
for _, f := range fns {
	go func() { v, err := f(ctx); ch <- out{v, err} }()
}
var errs []error
for range fns {
	o := <-ch
	if o.err == nil {
		cancel() // the others stop, but this code does not wait for them
		return o.v, nil
	}
	errs = append(errs, o.err)
}
return "", errors.Join(errs...)`,
  Race: `ctx, cancel := context.WithCancel(ctx)
defer cancel()
fns := []func(context.Context) (string, error){usage, invoices, taxRates}
type out struct {
	v   string
	err error
}
ch := make(chan out, len(fns))
var wg sync.WaitGroup
for _, f := range fns {
	wg.Add(1)
	go func() { defer wg.Done(); v, err := f(ctx); ch <- out{v, err} }()
}
first := <-ch
cancel()  // stop the others
wg.Wait() // and wait for them
return first.v, first.err`,
  AllSettled: `fns := []func(context.Context) (string, error){usage, invoices, taxRates}
values := make([]string, len(fns))
errs := make([]error, len(fns))
var wg sync.WaitGroup
for i, f := range fns {
	wg.Add(1)
	go func() {
		defer wg.Done()
		values[i], errs[i] = f(ctx)
	}()
}
wg.Wait()
// values[i] and errs[i] hold each result`,
};
