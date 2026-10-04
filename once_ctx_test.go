package monadz_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zkrebbekx/monadz"
)

func TestOnceWaiterContext(t *testing.T) {
	t.Run("Given a Once task whose first run blocks", func(t *testing.T) {
		release := make(chan struct{})
		started := make(chan struct{})
		task := monadz.NewTask(func(context.Context) (int, error) {
			close(started)
			<-release
			return 7, nil
		}).Once()

		first := task.Start(context.Background())
		<-started

		t.Run("When a second caller's context ends while it waits", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			start := time.Now()
			got := task.Run(ctx)

			t.Run("Then the second caller stops waiting with its own context error", func(t *testing.T) {
				if !errors.Is(got.Err(), context.DeadlineExceeded) {
					t.Fatalf("got %v, want context.DeadlineExceeded", got)
				}
				if waited := time.Since(start); waited > time.Second {
					t.Fatalf("waited %v, want about 20ms", waited)
				}
			})
		})

		close(release)
		t.Run("Then the first run still completes and its result is cached", func(t *testing.T) {
			if v := first.Await().Must(); v != 7 {
				t.Fatalf("got %d, want 7", v)
			}
			if v := task.Run(context.Background()).Must(); v != 7 {
				t.Fatalf("got %d, want cached 7", v)
			}
		})
	})
}
