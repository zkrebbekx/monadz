package eithers_test

import (
	"fmt"
	"slices"

	"github.com/zkrebbekx/monadz"
	"github.com/zkrebbekx/monadz/eithers"
)

// Partition splits the Left values from the Right values, in input order.
func ExamplePartition() {
	checks := []monadz.Either[string, int]{
		monadz.Right[string](10),
		monadz.Left[string, int]("upgrade"),
		monadz.Right[string](3),
	}

	paywalls, grants := eithers.Partition(slices.Values(checks))
	fmt.Println(paywalls, grants)
	// Output: [upgrade] [10 3]
}
