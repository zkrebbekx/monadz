package tasks_test

import (
	"context"
	"fmt"

	"github.com/zkrebbekx/monadz"
	"github.com/zkrebbekx/monadz/tasks"
)

// All runs the tasks at the same time and returns the values in input
// order. On the first failure, it cancels the other tasks and waits until
// they stop.
func ExampleAll() {
	square := func(n int) monadz.Task[int] {
		return monadz.NewTask(func(ctx context.Context) (int, error) {
			return n * n, nil
		})
	}

	res := tasks.All(square(1), square(2), square(3)).Run(context.Background())
	fmt.Println(res)
	// Output: Ok([1 4 9])
}
