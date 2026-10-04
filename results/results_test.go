package results_test

import (
	"errors"
	"iter"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/zkrebbekx/lift"
	"github.com/zkrebbekx/lift/results"
)

var (
	resErrA = errors.New("a")
	resErrB = errors.New("b")
	resErrC = errors.New("c")
)

func resEq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func resDeep(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// resCounting returns a sequence over rs and a pointer to the number of
// elements that the consumer read.
func resCounting[T any](rs ...T) (iter.Seq[T], *int) {
	n := 0
	return func(yield func(T) bool) {
		for _, r := range rs {
			n++
			if !yield(r) {
				return
			}
		}
	}, &n
}

// resPairs returns a Go (value, error) sequence.
func resPairs(vals []int, errs []error) iter.Seq2[int, error] {
	return func(yield func(int, error) bool) {
		for i := range vals {
			if !yield(vals[i], errs[i]) {
				return
			}
		}
	}
}

func TestZip(t *testing.T) {
	t.Run("Given Results of different types", func(t *testing.T) {
		t.Run("When Zip gets two Oks", func(t *testing.T) {
			got := results.Zip(lift.Ok(1), lift.Ok("a"))
			t.Run("Then it returns Ok of the Pair", func(t *testing.T) {
				resEq(t, got.Must(), lift.PairOf(1, "a"))
			})
		})
		t.Run("When Zip gets two failures", func(t *testing.T) {
			got := results.Zip(lift.Err[int](resErrA), lift.Err[string](resErrB))
			t.Run("Then the first failure wins", func(t *testing.T) {
				resEq(t, got.Err(), resErrA)
				resEq(t, results.Zip(lift.Ok(1), lift.Err[string](resErrB)).Err(), resErrB)
			})
		})
		t.Run("When Zip3 gets three Oks", func(t *testing.T) {
			got := results.Zip3(lift.Ok(1), lift.Ok("a"), lift.Ok(true))
			t.Run("Then it returns Ok of the Triple", func(t *testing.T) {
				resEq(t, got.Must(), lift.TripleOf(1, "a", true))
			})
		})
		t.Run("When Map3 gets failures in different positions", func(t *testing.T) {
			sum := func(a, b, c int) int { return a + b + c }
			t.Run("Then the first failure in argument order wins", func(t *testing.T) {
				resEq(t, results.Map3(lift.Ok(1), lift.Ok(2), lift.Ok(3), sum).Must(), 6)
				resEq(t, results.Map3(lift.Err[int](resErrA), lift.Err[int](resErrB), lift.Err[int](resErrC), sum).Err(), resErrA)
				resEq(t, results.Map3(lift.Ok(1), lift.Err[int](resErrB), lift.Err[int](resErrC), sum).Err(), resErrB)
				resEq(t, results.Map3(lift.Ok(1), lift.Ok(2), lift.Err[int](resErrC), sum).Err(), resErrC)
				resEq(t, results.Zip3(lift.Ok(1), lift.Ok(2), lift.Err[int](resErrC)).Err(), resErrC)
			})
			t.Run("Then f is not called on a failure", func(t *testing.T) {
				called := false
				results.Map3(lift.Ok(1), lift.Ok(2), lift.Err[int](resErrC), func(a, b, c int) int { called = true; return 0 })
				resEq(t, called, false)
			})
		})
	})
}

func TestFlattenTranspose(t *testing.T) {
	t.Run("Given nested Results", func(t *testing.T) {
		t.Run("When Flatten runs", func(t *testing.T) {
			t.Run("Then it removes one level, and an outer or inner failure stays", func(t *testing.T) {
				resEq(t, results.Flatten(lift.Ok(lift.Ok(1))).Must(), 1)
				resEq(t, results.Flatten(lift.Ok(lift.Err[int](resErrA))).Err(), resErrA)
				resEq(t, results.Flatten(lift.Err[lift.Result[int]](resErrB)).Err(), resErrB)
			})
		})
		t.Run("When Transpose runs on Ok(None)", func(t *testing.T) {
			got := results.Transpose(lift.Ok(lift.None[int]()))
			t.Run("Then it returns None", func(t *testing.T) {
				resEq(t, got.IsNone(), true)
			})
		})
		t.Run("When Transpose runs on Ok(Some(5))", func(t *testing.T) {
			got := results.Transpose(lift.Ok(lift.Some(5)))
			t.Run("Then it returns Some(Ok(5))", func(t *testing.T) {
				resEq(t, got.Must().Must(), 5)
			})
		})
		t.Run("When Transpose runs on Err(a)", func(t *testing.T) {
			got := results.Transpose(lift.Err[lift.Option[int]](resErrA))
			t.Run("Then it returns Some(Err(a))", func(t *testing.T) {
				resEq(t, got.Must().Err(), resErrA)
			})
		})
	})
}

func TestLiftCompose(t *testing.T) {
	t.Run("Given Go-shaped and Result-returning functions", func(t *testing.T) {
		atoi := results.Lift(strconv.Atoi)
		t.Run("When Lift wraps strconv.Atoi", func(t *testing.T) {
			t.Run("Then it returns Ok for a number and Err for text", func(t *testing.T) {
				resEq(t, atoi("7").Must(), 7)
				resEq(t, atoi("x").IsErr(), true)
			})
		})
		t.Run("When Compose joins two steps", func(t *testing.T) {
			calls := 0
			half := func(n int) lift.Result[int] {
				calls++
				if n%2 != 0 {
					return lift.Err[int](resErrA)
				}
				return lift.Ok(n / 2)
			}
			f := results.Compose(atoi, half)
			t.Run("Then the steps run in order and a failure skips the second step", func(t *testing.T) {
				resEq(t, f("8").Must(), 4)
				resEq(t, f("7").Err(), resErrA)
				resEq(t, calls, 2)
				resEq(t, f("x").IsErr(), true)
				resEq(t, calls, 2)
			})
		})
	})
}

func TestCollect(t *testing.T) {
	t.Run("Given a sequence of Results", func(t *testing.T) {
		t.Run("When every Result is Ok", func(t *testing.T) {
			seq, n := resCounting(lift.Ok(1), lift.Ok(2), lift.Ok(3))
			got := results.Collect(seq)
			t.Run("Then Collect returns Ok of all values", func(t *testing.T) {
				resDeep(t, got.Must(), []int{1, 2, 3})
				resEq(t, *n, 3)
			})
		})
		t.Run("When the second Result fails", func(t *testing.T) {
			seq, n := resCounting(lift.Ok(1), lift.Err[int](resErrA), lift.Ok(3), lift.Err[int](resErrB))
			got := results.Collect(seq)
			t.Run("Then Collect returns the first error and reads no further", func(t *testing.T) {
				resEq(t, got.Err(), resErrA)
				resEq(t, *n, 2)
			})
		})
		t.Run("When the sequence is empty", func(t *testing.T) {
			got := results.Collect(slices.Values([]lift.Result[int]{}))
			t.Run("Then Collect returns Ok of an empty, non-nil slice", func(t *testing.T) {
				resDeep(t, got.Must(), []int{})
			})
		})
	})
}

func TestSeq2(t *testing.T) {
	t.Run("Given a Go (value, error) sequence", func(t *testing.T) {
		vals := []int{1, 2, 3}
		t.Run("When FromSeq2 reads a sequence with no errors", func(t *testing.T) {
			got := results.FromSeq2(resPairs(vals, []error{nil, nil, nil}))
			t.Run("Then it returns Ok of all values", func(t *testing.T) {
				resDeep(t, got.Must(), []int{1, 2, 3})
			})
		})
		t.Run("When FromSeq2 reads a sequence with an error", func(t *testing.T) {
			got := results.FromSeq2(resPairs(vals, []error{nil, resErrA, resErrB}))
			t.Run("Then it returns the first error", func(t *testing.T) {
				resEq(t, got.Err(), resErrA)
			})
		})
		t.Run("When Seq converts the pairs", func(t *testing.T) {
			var got []string
			for r := range results.Seq(resPairs(vals, []error{nil, resErrA, nil})) {
				got = append(got, r.String())
			}
			t.Run("Then each pair becomes a Result", func(t *testing.T) {
				resDeep(t, got, []string{"Ok(1)", "Err(a)", "Ok(3)"})
			})
		})
		t.Run("When the consumer of Seq breaks early", func(t *testing.T) {
			read := 0
			src := func(yield func(int, error) bool) {
				for i := range 5 {
					read++
					if !yield(i, nil) {
						return
					}
				}
			}
			for range results.Seq(src) {
				break
			}
			t.Run("Then Seq stops reading the source", func(t *testing.T) {
				resEq(t, read, 1)
			})
		})
	})
}

func TestTraverseFold(t *testing.T) {
	t.Run("Given a sequence of strings", func(t *testing.T) {
		t.Run("When Traverse parses valid numbers", func(t *testing.T) {
			got := results.Traverse(slices.Values([]string{"1", "2"}), results.Lift(strconv.Atoi))
			t.Run("Then it returns Ok of all numbers", func(t *testing.T) {
				resDeep(t, got.Must(), []int{1, 2})
			})
		})
		t.Run("When Traverse meets a failure", func(t *testing.T) {
			calls := 0
			f := func(s string) lift.Result[int] {
				calls++
				if s == "bad" {
					return lift.Err[int](resErrA)
				}
				return lift.Ok(len(s))
			}
			got := results.Traverse(slices.Values([]string{"x", "bad", "y"}), f)
			t.Run("Then it returns the error and does not call f again", func(t *testing.T) {
				resEq(t, got.Err(), resErrA)
				resEq(t, calls, 2)
			})
		})
		t.Run("When TraverseFunc runs strconv.Atoi", func(t *testing.T) {
			t.Run("Then valid input gives Ok and invalid input gives Err", func(t *testing.T) {
				resDeep(t, results.TraverseFunc(slices.Values([]string{"3", "4"}), strconv.Atoi).Must(), []int{3, 4})
				resEq(t, results.TraverseFunc(slices.Values([]string{"3", "x"}), strconv.Atoi).IsErr(), true)
			})
		})
		t.Run("When Fold sums numbers", func(t *testing.T) {
			add := func(acc int, s string) lift.Result[int] {
				return lift.Of(strconv.Atoi(s)).Map(func(n int) int { return acc + n })
			}
			t.Run("Then it returns the sum, or the first failure", func(t *testing.T) {
				resEq(t, results.Fold(slices.Values([]string{"1", "2", "3"}), 10, add).Must(), 16)
				resEq(t, results.Fold(slices.Values([]string{"1", "x", "3"}), 10, add).IsErr(), true)
				resEq(t, results.Fold(slices.Values([]string{}), 10, add).Must(), 10)
			})
		})
	})
}

func TestPartitionOksErrs(t *testing.T) {
	t.Run("Given a mixed sequence of Results", func(t *testing.T) {
		rs := []lift.Result[int]{lift.Ok(1), lift.Err[int](resErrA), lift.Ok(2), lift.Err[int](resErrB)}
		t.Run("When Partition runs", func(t *testing.T) {
			oks, errs := results.Partition(slices.Values(rs))
			t.Run("Then it splits the values and errors in order", func(t *testing.T) {
				resDeep(t, oks, []int{1, 2})
				resDeep(t, errs, []error{resErrA, resErrB})
			})
		})
		t.Run("When Oks and Errs are collected", func(t *testing.T) {
			t.Run("Then Oks gives the values and Errs gives the errors", func(t *testing.T) {
				resDeep(t, slices.Collect(results.Oks(slices.Values(rs))), []int{1, 2})
				resDeep(t, slices.Collect(results.Errs(slices.Values(rs))), []error{resErrA, resErrB})
			})
		})
		t.Run("When the consumer of Oks and Errs breaks early", func(t *testing.T) {
			okSeq, okRead := resCounting(rs...)
			var gotOk []int
			for v := range results.Oks(okSeq) {
				gotOk = append(gotOk, v)
				break
			}
			errSeq, errRead := resCounting(rs...)
			var gotErr []error
			for e := range results.Errs(errSeq) {
				gotErr = append(gotErr, e)
				break
			}
			t.Run("Then each stops reading at the first yielded element", func(t *testing.T) {
				resDeep(t, gotOk, []int{1})
				resEq(t, *okRead, 1)
				resDeep(t, gotErr, []error{resErrA})
				resEq(t, *errRead, 2)
			})
		})
	})
}

func TestFirst(t *testing.T) {
	t.Run("Given First with different arguments", func(t *testing.T) {
		t.Run("When one Result is Ok", func(t *testing.T) {
			got := results.First(lift.Err[int](resErrA), lift.Ok(2), lift.Ok(3))
			t.Run("Then it returns the first Ok", func(t *testing.T) {
				resEq(t, got.Must(), 2)
			})
		})
		t.Run("When every Result fails", func(t *testing.T) {
			got := results.First(lift.Err[int](resErrA), lift.Err[int](resErrB))
			t.Run("Then it returns all errors joined", func(t *testing.T) {
				resEq(t, errors.Is(got.Err(), resErrA), true)
				resEq(t, errors.Is(got.Err(), resErrB), true)
				resEq(t, got.Err().Error(), "a\nb")
			})
		})
		t.Run("When there are no arguments", func(t *testing.T) {
			got := results.First[int]()
			t.Run("Then it returns an error", func(t *testing.T) {
				resEq(t, got.IsErr(), true)
				resEq(t, got.Err().Error(), "results: First with no arguments")
			})
		})
	})
}
