package monadz_test

import (
	"fmt"
	"testing"

	"github.com/zkrebbekx/monadz"
)

// eitConfig is a shared environment for Reader tests.
type eitConfig struct {
	Host   string
	Port   int
	Prefix string
}

func eitAddr() monadz.Reader[eitConfig, string] {
	return monadz.Asks(func(c eitConfig) string { return fmt.Sprintf("%s:%d", c.Host, c.Port) })
}

func TestReaderPrimitives(t *testing.T) {
	t.Run("Given an environment", func(t *testing.T) {
		env := eitConfig{Host: "db", Port: 5432, Prefix: "/v1"}
		t.Run("When NewReader wraps a function", func(t *testing.T) {
			r := monadz.NewReader(func(c eitConfig) int { return c.Port + 1 })
			t.Run("Then Run calls it with the environment", func(t *testing.T) {
				eitEq(t, r.Run(env), 5433)
			})
		})
		t.Run("When ReaderOf runs", func(t *testing.T) {
			t.Run("Then it ignores the environment", func(t *testing.T) {
				eitEq(t, monadz.ReaderOf[eitConfig]("const").Run(env), "const")
			})
		})
		t.Run("When Ask runs", func(t *testing.T) {
			t.Run("Then the value is the environment", func(t *testing.T) {
				eitEq(t, monadz.Ask[eitConfig]().Run(env), env)
			})
		})
		t.Run("When Asks runs", func(t *testing.T) {
			t.Run("Then the value is the projection of the environment", func(t *testing.T) {
				eitEq(t, eitAddr().Run(env), "db:5432")
			})
		})
		t.Run("When a Reader is built and not run", func(t *testing.T) {
			calls := 0
			r := monadz.NewReader(func(eitConfig) int { calls++; return 0 }).Map(eitDouble)
			before := calls
			r.Run(env)
			t.Run("Then nothing runs until Run", func(t *testing.T) {
				eitEq(t, before, 0)
				eitEq(t, calls, 1)
			})
		})
	})
}

func TestReaderCombinators(t *testing.T) {
	t.Run("Given an environment and an address Reader", func(t *testing.T) {
		env := eitConfig{Host: "api", Port: 8080, Prefix: "/v2"}
		t.Run("When Map formats the value", func(t *testing.T) {
			got := eitAddr().Map(func(a string) string { return "http://" + a }).Run(env)
			t.Run("Then the value is mapped", func(t *testing.T) {
				eitEq(t, got, "http://api:8080")
			})
		})
		t.Run("When AndThen builds a URL from the address", func(t *testing.T) {
			var seen []eitConfig
			url := eitAddr().AndThen(func(addr string) monadz.Reader[eitConfig, string] {
				return monadz.NewReader(func(c eitConfig) string {
					seen = append(seen, c)
					return "http://" + addr + c.Prefix
				})
			})
			got := url.Run(env)
			t.Run("Then the same environment flows to the second step", func(t *testing.T) {
				eitEq(t, got, "http://api:8080/v2")
				eitEq(t, seen, []eitConfig{env})
			})
		})
		t.Run("When Local changes the port for an inner Reader", func(t *testing.T) {
			admin := eitAddr().Local(func(c eitConfig) eitConfig { c.Port = 9000; return c })
			both := admin.AndThen(func(a string) monadz.Reader[eitConfig, [2]string] {
				return eitAddr().Map(func(b string) [2]string { return [2]string{a, b} })
			})
			got := both.Run(env)
			t.Run("Then only the inner Reader sees the changed environment", func(t *testing.T) {
				eitEq(t, got, [2]string{"api:9000", "api:8080"})
				eitEq(t, env.Port, 8080)
			})
		})
	})
}
