package validations_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zkrebbekx/monadz"
	"github.com/zkrebbekx/monadz/validations"
)

// Map3 runs every check and collects ALL the errors. Field puts the field
// name on each error, so an API can report it next to the field.
func ExampleMap3() {
	type signup struct {
		Email, Password, Company string
	}
	email := func(s string) monadz.Validation[string] {
		if !strings.Contains(s, "@") {
			return monadz.Invalid[string](errors.New("not an email"))
		}
		return monadz.Valid(s)
	}
	password := func(s string) monadz.Validation[string] {
		if len(s) < 8 {
			return monadz.Invalid[string](errors.New("too short"))
		}
		return monadz.Valid(s)
	}
	company := func(s string) monadz.Validation[string] { return monadz.Valid(s) }

	form := validations.Map3(
		email("ann.example.com").Field("email"),
		password("hunter2").Field("password"),
		company("Acme").Field("company"),
		func(e, p, c string) signup { return signup{e, p, c} },
	)

	for _, err := range form.Errors() {
		fmt.Println(err)
	}
	// Output:
	// email: not an email
	// password: too short
}
