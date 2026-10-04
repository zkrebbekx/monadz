package monadz_test

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zkrebbekx/monadz"
)

var eitErrLoad = errors.New("load failed")

const eitZeroLazyMsg = "monadz: Get on a zero Lazy; use NewLazy"

func TestLazyRunsOnce(t *testing.T) {
	t.Run("Given a Lazy that counts its calls", func(t *testing.T) {
		t.Run("When nothing calls Get", func(t *testing.T) {
			var calls atomic.Int32
			_ = monadz.NewLazy(func() int { calls.Add(1); return 42 })
			t.Run("Then the function does not run", func(t *testing.T) {
				wantDeep(t, calls.Load(), int32(0))
			})
		})

		t.Run("When 50 goroutines call Get at the same time", func(t *testing.T) {
			var calls atomic.Int32
			l := monadz.NewLazy(func() int { calls.Add(1); return 42 })
			const n = 50
			results := make([]int, n)
			var start, done sync.WaitGroup
			start.Add(1)
			for i := range n {
				done.Go(func() {
					start.Wait()
					results[i] = l.Get()
				})
			}
			start.Done()
			done.Wait()
			t.Run("Then the function runs exactly one time", func(t *testing.T) {
				wantDeep(t, calls.Load(), int32(1))
			})
			t.Run("Then every caller sees the same value", func(t *testing.T) {
				for i, r := range results {
					if r != 42 {
						t.Errorf("results[%d]: got %d, want 42", i, r)
					}
				}
			})
			t.Run("Then a later Get returns the cached value without a new call", func(t *testing.T) {
				wantDeep(t, l.Get(), 42)
				wantDeep(t, calls.Load(), int32(1))
			})
		})

		t.Run("When a copy of the Lazy calls Get", func(t *testing.T) {
			var calls atomic.Int32
			l := monadz.NewLazy(func() int { calls.Add(1); return 42 })
			cp := l
			cp.Get()
			l.Get()
			t.Run("Then the copy shares the cache", func(t *testing.T) {
				wantDeep(t, calls.Load(), int32(1))
			})
		})
	})
}

func TestLazyCombinators(t *testing.T) {
	t.Run("Given a source Lazy with a call counter", func(t *testing.T) {
		t.Run("When Map is built", func(t *testing.T) {
			var src, step atomic.Int32
			base := monadz.NewLazy(func() int { src.Add(1); return 10 })
			m := base.Map(func(n int) string { step.Add(1); return string(rune('A' + n)) })
			t.Run("Then nothing runs before Get", func(t *testing.T) {
				wantDeep(t, src.Load()+step.Load(), int32(0))
			})
			t.Run("Then Get runs each function one time and caches", func(t *testing.T) {
				wantDeep(t, m.Get(), "K")
				wantDeep(t, m.Get(), "K")
				wantDeep(t, src.Load(), int32(1))
				wantDeep(t, step.Load(), int32(1))
			})
		})

		t.Run("When AndThen is built", func(t *testing.T) {
			var src, step, inner atomic.Int32
			base := monadz.NewLazy(func() int { src.Add(1); return 10 })
			a := base.AndThen(func(n int) monadz.Lazy[int] {
				step.Add(1)
				return monadz.NewLazy(func() int { inner.Add(1); return n + 1 })
			})
			t.Run("Then nothing runs before Get", func(t *testing.T) {
				wantDeep(t, src.Load()+step.Load()+inner.Load(), int32(0))
			})
			t.Run("Then Get gives the inner value, and every function runs one time", func(t *testing.T) {
				wantDeep(t, a.Get(), 11)
				wantDeep(t, a.Get(), 11)
				wantDeep(t, []int32{src.Load(), step.Load(), inner.Load()}, []int32{1, 1, 1})
			})
		})

		t.Run("When ZipWith combines it with a second Lazy", func(t *testing.T) {
			var src, other, step atomic.Int32
			base := monadz.NewLazy(func() int { src.Add(1); return 10 })
			o := monadz.NewLazy(func() string { other.Add(1); return "x" })
			z := base.ZipWith(o, func(n int, s string) string {
				step.Add(1)
				return s + string(rune('0'+n-10))
			})
			t.Run("Then nothing runs before Get", func(t *testing.T) {
				wantDeep(t, src.Load()+other.Load()+step.Load(), int32(0))
			})
			t.Run("Then Get combines both values one time", func(t *testing.T) {
				wantDeep(t, z.Get(), "x0")
				wantDeep(t, z.Get(), "x0")
				wantDeep(t, []int32{src.Load(), other.Load(), step.Load()}, []int32{1, 1, 1})
			})
			t.Run("Then the source Lazy shares its cache with the zip", func(t *testing.T) {
				wantDeep(t, base.Get(), 10)
				wantDeep(t, src.Load(), int32(1))
			})
		})
	})
}

func TestLazyOf(t *testing.T) {
	t.Run("Given a LazyOf that fails", func(t *testing.T) {
		var calls atomic.Int32
		l := monadz.LazyOf(func() (int, error) { calls.Add(1); return 0, eitErrLoad })
		t.Run("When Get is called two times", func(t *testing.T) {
			r1 := l.Get()
			r2 := l.Get()
			t.Run("Then both results hold the error and the function ran one time", func(t *testing.T) {
				wantTrue(t, r1.IsErr(), "r1.IsErr()")
				wantTrue(t, errors.Is(r1.Err(), eitErrLoad), "errors.Is(r1.Err(), eitErrLoad)")
				wantTrue(t, errors.Is(r2.Err(), eitErrLoad), "errors.Is(r2.Err(), eitErrLoad)")
				wantDeep(t, calls.Load(), int32(1))
			})
		})
	})

	t.Run("Given a LazyOf that succeeds", func(t *testing.T) {
		var calls atomic.Int32
		l := monadz.LazyOf(func() (string, error) { calls.Add(1); return "cfg", nil })
		t.Run("When nothing calls Get", func(t *testing.T) {
			t.Run("Then the function does not run", func(t *testing.T) {
				wantDeep(t, calls.Load(), int32(0))
			})
		})
		t.Run("When Get is called two times", func(t *testing.T) {
			r1, r2 := l.Get(), l.Get()
			t.Run("Then it returns Ok and runs one time", func(t *testing.T) {
				wantDeep(t, r1, monadz.Ok("cfg"))
				wantDeep(t, r2, monadz.Ok("cfg"))
				wantDeep(t, calls.Load(), int32(1))
			})
		})
	})
}

func TestLazyPanics(t *testing.T) {
	t.Run("Given a Lazy whose function panics", func(t *testing.T) {
		t.Run("When Get is called three times", func(t *testing.T) {
			var calls atomic.Int32
			l := monadz.NewLazy(func() int { calls.Add(1); panic("boom") })
			t.Run("Then every Get panics with the same value and the function runs one time", func(t *testing.T) {
				wantPanic(t, func() { l.Get() }, "boom")
				wantPanic(t, func() { l.Get() }, "boom")
				wantPanic(t, func() { l.Get() }, "boom")
				wantDeep(t, calls.Load(), int32(1))
			})
		})
		t.Run("When a Map of it calls Get", func(t *testing.T) {
			l := monadz.NewLazy(func() int { panic("boom") })
			m := l.Map(func(n int) int { return n })
			t.Run("Then the Map panics too, on every Get", func(t *testing.T) {
				wantPanic(t, func() { m.Get() }, "boom")
				wantPanic(t, func() { m.Get() }, "boom")
			})
		})
	})

	t.Run("Given a zero Lazy", func(t *testing.T) {
		var z monadz.Lazy[int]
		t.Run("When Get is called", func(t *testing.T) {
			t.Run("Then it panics with a clear message", func(t *testing.T) {
				wantPanic(t, func() { z.Get() }, eitZeroLazyMsg)
			})
		})
		t.Run("When it is mapped", func(t *testing.T) {
			var m monadz.Lazy[int]
			wantNoPanic(t, func() { m = z.Map(func(n int) int { return n }) })
			t.Run("Then building does not panic, but Get does", func(t *testing.T) {
				wantPanic(t, func() { m.Get() }, eitZeroLazyMsg)
			})
		})
	})
}

func TestLazyGoexit(t *testing.T) {
	t.Run("Given a Lazy whose function calls runtime.Goexit", func(t *testing.T) {
		l := monadz.NewLazy(func() int { runtime.Goexit(); return 1 })
		t.Run("When Get is called in a goroutine, and then again", func(t *testing.T) {
			done := make(chan any, 1)
			go func() {
				defer func() { done <- recover() }()
				l.Get()
			}()
			first := <-done
			second := recovered(func() { l.Get() })
			t.Run("Then both calls panic with a *runtime.PanicNilError", func(t *testing.T) {
				_, ok1 := first.(*runtime.PanicNilError)
				_, ok2 := second.(*runtime.PanicNilError)
				wantTrue(t, ok1, "the first panic is a *runtime.PanicNilError")
				wantTrue(t, ok2, "the second panic is a *runtime.PanicNilError")
			})
		})
	})
}
