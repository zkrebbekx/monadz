package tasks_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/zkrebbekx/lift/tasks"
)

func TestTraverseReadsItemsAtRun(t *testing.T) {
	t.Run("Given a Traverse over a sequence that counts its reads", func(t *testing.T) {
		reads := 0
		items := func(yield func(int) bool) {
			reads++
			for _, v := range []int{1, 2, 3} {
				if !yield(v) {
					return
				}
			}
		}
		task := tasks.Traverse(items, 2, func(_ context.Context, n int) (int, error) { return n * 10, nil })

		t.Run("When the Task is only built", func(t *testing.T) {
			t.Run("Then the sequence is not read", func(t *testing.T) {
				if reads != 0 {
					t.Fatalf("got %d reads, want 0", reads)
				}
			})
		})

		t.Run("When the Task runs two times", func(t *testing.T) {
			a := task.Run(context.Background()).Must()
			b := task.Run(context.Background()).Must()

			t.Run("Then each run reads the sequence again and gives the same output", func(t *testing.T) {
				if reads != 2 {
					t.Fatalf("got %d reads, want 2", reads)
				}
				want := []int{10, 20, 30}
				if !reflect.DeepEqual(a, want) || !reflect.DeepEqual(b, want) {
					t.Fatalf("got %v and %v, want %v", a, b, want)
				}
			})
		})
	})
}
