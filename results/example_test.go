package results_test

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/zkrebbekx/monadz/results"
)

// TraverseFunc takes a Go-shaped function, such as strconv.Atoi. It returns
// Ok of every value, or the first error.
func ExampleTraverseFunc() {
	fmt.Println(results.TraverseFunc(slices.Values([]string{"80", "443"}), strconv.Atoi))
	fmt.Println(results.TraverseFunc(slices.Values([]string{"80", "x"}), strconv.Atoi))
	// Output:
	// Ok([80 443])
	// Err(strconv.Atoi: parsing "x": invalid syntax)
}
