package scenarios

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"unicode"

	"github.com/zkrebbekx/monadz"
	"github.com/zkrebbekx/monadz/validations"
)

// Signup errors.
var (
	ErrNotEmail   = errors.New("not an email address")
	ErrTooShort   = errors.New("must have at least 8 characters")
	ErrNoDigit    = errors.New("must have a digit")
	ErrRequired   = errors.New("is required")
	ErrTooLong    = errors.New("must have at most 40 characters")
	ErrEmailTaken = errors.New("email is already registered")
	ErrDatabase   = errors.New("database is not available")
)

// SignupInput is the input of the Result and Validation scenarios.
type SignupInput struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	Company    string `json:"company"`
	EmailTaken bool   `json:"emailTaken"`
	DBDown     bool   `json:"dbDown"`
}

// SignupForm is the form after the checks.
type SignupForm struct {
	Email, Password, Company string
}

// NewAccount is an account that is ready to insert.
type NewAccount struct {
	Email, PasswordHash, Company string
}

// Account is a stored account.
type Account struct {
	ID      string
	Email   string
	Company string
}

func isEmail(s string) bool {
	user, domain, ok := strings.Cut(strings.TrimSpace(s), "@")
	return ok && user != "" && strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".")
}

func longEnough(s string) bool { return len(s) >= 8 }

func hasDigit(s string) bool { return strings.ContainsFunc(s, unicode.IsDigit) }

func present(s string) bool { return strings.TrimSpace(s) != "" }

func shortEnough(s string) bool { return len(strings.TrimSpace(s)) <= 40 }

func fieldErr(field string, err error) error { return &monadz.FieldError{Field: field, Err: err} }

// validateSignup is the fail-fast check for the Result pipeline. Each Ensure
// runs only if the one before it passed, so the caller sees one error.
func validateSignup(f SignupForm) (SignupForm, error) {
	return monadz.Ok(f).
		Ensure(func(f SignupForm) bool { return isEmail(f.Email) }, fieldErr("email", ErrNotEmail)).
		Ensure(func(f SignupForm) bool { return longEnough(f.Password) }, fieldErr("password", ErrTooShort)).
		Ensure(func(f SignupForm) bool { return hasDigit(f.Password) }, fieldErr("password", ErrNoDigit)).
		Ensure(func(f SignupForm) bool { return present(f.Company) }, fieldErr("company", ErrRequired)).
		Ensure(func(f SignupForm) bool { return shortEnough(f.Company) }, fieldErr("company", ErrTooLong)).
		Get()
}

// fakeDB is the account store of the story.
type fakeDB struct{ emailTaken, down bool }

func (db fakeDB) checkEmailFree(f SignupForm) (SignupForm, error) {
	if db.emailTaken {
		return f, fieldErr("email", ErrEmailTaken)
	}
	return f, nil
}

func (db fakeDB) insert(a NewAccount) (Account, error) {
	if db.down {
		return Account{}, ErrDatabase
	}
	return Account{ID: "acct_1042", Email: a.Email, Company: a.Company}, nil
}

// hashPassword makes a demo hash. A real service uses bcrypt or argon2id.
func hashPassword(f SignupForm) NewAccount {
	sum := sha256.Sum256([]byte(f.Password))
	return NewAccount{Email: strings.TrimSpace(f.Email), PasswordHash: "sha256:" + hex.EncodeToString(sum[:])[:12] + "…", Company: strings.TrimSpace(f.Company)}
}

// statusFor maps an error to an HTTP status.
func statusFor(err error) int {
	switch {
	case errors.Is(err, ErrEmailTaken):
		return http.StatusConflict
	case errors.Is(err, ErrDatabase):
		return http.StatusServiceUnavailable
	default:
		var fe *monadz.FieldError
		if errors.As(err, &fe) {
			return http.StatusUnprocessableEntity
		}
		return http.StatusInternalServerError
	}
}

// ResultTrace is the result of the Result scenario.
type ResultTrace struct {
	Steps      []Step `json:"steps"`
	Status     int    `json:"status"`
	StatusText string `json:"statusText"`
	Body       string `json:"body"`
}

// RunResult runs the signup pipeline. A failed step moves the value to the
// Err track, and the later steps do not run.
//
// The chain is:
//
//	monadz.Ok(form).Then(validateSignup).Then(db.checkEmailFree).
//		Map(hashPassword).Then(db.insert)
func RunResult(in SignupInput) ResultTrace {
	db := fakeDB{emailTaken: in.EmailTaken, down: in.DBDown}
	form := monadz.Ok(SignupForm{Email: in.Email, Password: in.Password, Company: in.Company})

	valid := form.Then(validateSignup)
	free := valid.Then(db.checkEmailFree)
	hashed := free.Map(hashPassword)
	account := hashed.Then(db.insert)

	showForm := func(f SignupForm) string { return f.Email }
	steps := []Step{
		resStep("validate", form, valid, showForm, showForm),
		resStep("check email is free", valid, free, showForm, showForm),
		resStep("hash password", free, hashed, showForm, func(a NewAccount) string { return a.PasswordHash }),
		resStep("insert account", hashed, account, func(a NewAccount) string { return a.Email }, func(a Account) string { return a.ID }),
	}

	status := account.Match(
		func(Account) int { return http.StatusCreated },
		statusFor,
	)
	body := account.Match(
		func(a Account) string { return `{"id":"` + a.ID + `","company":` + quote(a.Company) + `}` },
		func(err error) string { return `{"error":` + quote(err.Error()) + `}` },
	)
	return ResultTrace{Steps: steps, Status: status, StatusText: http.StatusText(status), Body: body}
}

// resStep describes one Result step: skipped if the input was Err, else Ok
// or Err.
func resStep[T, U any](name string, in monadz.Result[T], out monadz.Result[U], showIn func(T) string, showOut func(U) string) Step {
	s := Step{Name: name}
	v, err := in.Get()
	if err != nil {
		s.Input, s.Output, s.Track = "Err", "Err", TrackSkipped
		s.Note = "An earlier step failed, so this step does not run."
		return s
	}
	s.Input = showIn(v)
	if u, err := out.Get(); err != nil {
		s.Track, s.Output, s.Note = TrackErr, "Err("+err.Error()+")", err.Error()
	} else {
		s.Track, s.Output = TrackOk, "Ok("+showOut(u)+")"
	}
	return s
}

// ---- Validation ----

func validateEmail(s string) monadz.Validation[string] {
	return monadz.ValidIf(strings.TrimSpace(s), isEmail(s), ErrNotEmail)
}

func validatePassword(s string) monadz.Validation[string] {
	// Map2 runs both checks, so a password can have two errors.
	return validations.Map2(
		monadz.ValidIf(s, longEnough(s), ErrTooShort),
		monadz.ValidIf(s, hasDigit(s), ErrNoDigit),
		func(p, _ string) string { return p },
	)
}

func validateCompany(s string) monadz.Validation[string] {
	return monadz.ValidIf(strings.TrimSpace(s), present(s), ErrRequired).
		Ensure(shortEnough, ErrTooLong)
}

// FieldMessage is one error about one field.
type FieldMessage struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationTrace is the result of the Validation scenario.
type ValidationTrace struct {
	Valid        bool           `json:"valid"`
	Summary      string         `json:"summary"`
	Errors       []FieldMessage `json:"errors"`
	ResultErrors []FieldMessage `json:"resultErrors"`
	Status       int            `json:"status"`
}

// RunValidation checks every field and collects ALL the errors. It also
// runs the fail-fast Result check on the same form, for comparison.
func RunValidation(in SignupInput) ValidationTrace {
	signup := validations.Map3(
		validateEmail(in.Email).Field("email"),
		validatePassword(in.Password).Field("password"),
		validateCompany(in.Company).Field("company"),
		func(email, password, company string) SignupForm {
			return SignupForm{Email: email, Password: password, Company: company}
		},
	)

	t := ValidationTrace{Valid: signup.IsValid(), Errors: []FieldMessage{}, ResultErrors: []FieldMessage{}, Status: http.StatusOK}
	for _, err := range signup.Errors() {
		t.Errors = append(t.Errors, fieldMessage(err))
	}
	if signup.IsValid() {
		t.Summary = "Valid(" + strings.TrimSpace(in.Email) + ")"
	} else {
		t.Summary = signup.String()
		t.Status = http.StatusUnprocessableEntity
	}

	if _, err := validateSignup(SignupForm{Email: in.Email, Password: in.Password, Company: in.Company}); err != nil {
		t.ResultErrors = append(t.ResultErrors, fieldMessage(err))
	}
	return t
}

func fieldMessage(err error) FieldMessage {
	var fe *monadz.FieldError
	if errors.As(err, &fe) {
		return FieldMessage{Field: fe.Field, Message: fe.Err.Error()}
	}
	return FieldMessage{Message: err.Error()}
}
