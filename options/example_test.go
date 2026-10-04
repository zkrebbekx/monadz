package options_test

import (
	"fmt"
	"slices"

	"github.com/zkrebbekx/monadz"
	"github.com/zkrebbekx/monadz/options"
)

// Traverse returns Some of every value, or None if one value is None.
func ExampleTraverse() {
	portOf := map[string]int{"http": 80, "https": 443}
	lookup := func(name string) monadz.Option[int] { return monadz.Lookup(portOf, name) }

	fmt.Println(options.Traverse(slices.Values([]string{"http", "https"}), lookup))
	fmt.Println(options.Traverse(slices.Values([]string{"http", "gopher"}), lookup))
	// Output:
	// Some([80 443])
	// None
}

// Zip joins two Options into one Option of a Pair.
func ExampleZip() {
	fmt.Println(options.Zip(monadz.Some("x"), monadz.Some(1)))
	fmt.Println(options.Zip(monadz.Some("x"), monadz.None[int]()))
	// Output:
	// Some((x, 1))
	// None
}
