package lift_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zkrebbekx/lift"
)

// eitAudit returns a Writer step that charges an amount and logs it.
func eitAudit(total, amount int) lift.Writer[string, int] {
	return lift.NewWriter(total+amount, fmt.Sprintf("charge %d", amount))
}

func TestWriterPrimitives(t *testing.T) {
	t.Run("Given NewWriter with a value and two log entries", func(t *testing.T) {
		w := lift.NewWriter(7, "a", "b")
		t.Run("When Run is called", func(t *testing.T) {
			v, log := w.Run()
			t.Run("Then it returns the value and the log", func(t *testing.T) {
				eitEq(t, v, 7)
				eitEq(t, log, []string{"a", "b"})
			})
		})
		t.Run("When the log from one Run is mutated", func(t *testing.T) {
			_, log := w.Run()
			log[0] = "changed"
			_ = append(log[:1], "appended")
			_, again := w.Run()
			t.Run("Then a second Run returns the original log", func(t *testing.T) {
				eitEq(t, again, []string{"a", "b"})
			})
		})
	})

	t.Run("Given NewWriter with no log entries", func(t *testing.T) {
		w := lift.NewWriter[string](1)
		t.Run("When Run is called", func(t *testing.T) {
			_, log := w.Run()
			t.Run("Then the log is empty", func(t *testing.T) {
				eitEq(t, len(log), 0)
			})
		})
	})

	t.Run("Given a caller slice passed to NewWriter", func(t *testing.T) {
		entries := []string{"x", "y"}
		w := lift.NewWriter(0, entries...)
		t.Run("When the caller mutates the slice after the call", func(t *testing.T) {
			entries[0] = "mutated"
			_, log := w.Run()
			t.Run("Then the Writer log does not change", func(t *testing.T) {
				eitEq(t, log, []string{"x", "y"})
			})
		})
	})

	t.Run("Given Tell with two entries", func(t *testing.T) {
		w := lift.Tell("start", "end")
		t.Run("When Run is called", func(t *testing.T) {
			v, log := w.Run()
			t.Run("Then the value is Unit and the log has the entries", func(t *testing.T) {
				eitEq(t, v, lift.Unit{})
				eitEq(t, log, []string{"start", "end"})
			})
		})
	})
}

func TestWriterCombinators(t *testing.T) {
	t.Run("Given an audit Writer that charges 10", func(t *testing.T) {
		w := eitAudit(0, 10)
		t.Run("When Map doubles the value", func(t *testing.T) {
			v, log := w.Map(eitDouble).Run()
			t.Run("Then the value changes and the log does not", func(t *testing.T) {
				eitEq(t, v, 20)
				eitEq(t, log, []string{"charge 10"})
			})
		})
		t.Run("When AndThen chains two more charges and a Tell", func(t *testing.T) {
			total := w.AndThen(func(t int) lift.Writer[string, int] {
				return eitAudit(t, 5)
			}).AndThen(func(t int) lift.Writer[string, int] {
				return lift.Tell("checkpoint").Map(func(lift.Unit) int { return t })
			}).AndThen(func(t int) lift.Writer[string, int] {
				return eitAudit(t, 1)
			})
			v, log := total.Run()
			t.Run("Then the value is the sum and the logs join in order", func(t *testing.T) {
				eitEq(t, v, 16)
				eitEq(t, log, []string{"charge 10", "charge 5", "checkpoint", "charge 1"})
			})
			t.Run("Then a second Run gives the same log", func(t *testing.T) {
				_, again := total.Run()
				eitEq(t, again, log)
			})
		})
		t.Run("When Censor removes entries from the log", func(t *testing.T) {
			c := lift.NewWriter(1, "user=alice", "ok").Censor(func(log []string) []string {
				out := make([]string, 0, len(log))
				for _, e := range log {
					if !strings.HasPrefix(e, "user=") {
						out = append(out, e)
					}
				}
				return out
			})
			v, log := c.Run()
			t.Run("Then the value stays and the log is changed", func(t *testing.T) {
				eitEq(t, v, 1)
				eitEq(t, log, []string{"ok"})
			})
		})
		t.Run("When Censor changes the log slice in place", func(t *testing.T) {
			base := lift.NewWriter(1, "secret", "ok")
			c := base.Censor(func(log []string) []string {
				log[0] = "***"
				return log
			})
			_, censored := c.Run()
			_, original := base.Run()
			t.Run("Then the censored log changes and the base Writer log does not", func(t *testing.T) {
				eitEq(t, censored, []string{"***", "ok"})
				eitEq(t, original, []string{"secret", "ok"})
			})
		})
	})
}
