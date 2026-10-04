package monadz_test

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"strconv"
	"testing"

	"github.com/zkrebbekx/monadz"
)

type optPatchReq struct {
	Name monadz.Patch[string] `json:"name,omitzero"`
	Age  monadz.Patch[int]    `json:"age,omitzero"`
}

func TestPatchStates(t *testing.T) {
	t.Run("Given the three Patch constructors", func(t *testing.T) {
		set, null, missing := monadz.PatchSet(5), monadz.PatchNull[int](), monadz.PatchMissing[int]()
		t.Run("When the states are queried", func(t *testing.T) {
			t.Run("Then each Patch reports only its own state", func(t *testing.T) {
				wantDeep(t, []bool{set.IsSet(), set.IsNull(), set.IsMissing()}, []bool{true, false, false})
				wantDeep(t, []bool{null.IsSet(), null.IsNull(), null.IsMissing()}, []bool{false, true, false})
				wantDeep(t, []bool{missing.IsSet(), missing.IsNull(), missing.IsMissing()}, []bool{false, false, true})
			})
			t.Run("Then only Missing is zero", func(t *testing.T) {
				wantDeep(t, missing.IsZero(), true)
				wantDeep(t, null.IsZero(), false)
				wantDeep(t, set.IsZero(), false)
				wantDeep(t, monadz.PatchSet(0).IsZero(), false)
			})
			t.Run("Then the zero value of Patch is Missing", func(t *testing.T) {
				var p monadz.Patch[int]
				wantDeep(t, p.IsMissing(), true)
				wantDeep(t, p, missing)
			})
		})
		t.Run("When Value is called", func(t *testing.T) {
			t.Run("Then Set gives Some and the others give None", func(t *testing.T) {
				wantDeep(t, set.Option(), monadz.Some(5))
				wantDeep(t, monadz.PatchSet(0).Option(), monadz.Some(0))
				wantDeep(t, null.Option().IsNone(), true)
				wantDeep(t, missing.Option().IsNone(), true)
			})
		})
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then it gives Set(<value>), Null, or Missing", func(t *testing.T) {
				wantDeep(t, set.String(), "Set(5)")
				wantDeep(t, null.String(), "Null")
				wantDeep(t, fmt.Sprint(missing), "Missing")
			})
		})
	})
}

func TestPatchApply(t *testing.T) {
	t.Run("Given a stored value of 9", func(t *testing.T) {
		t.Run("When Apply is called", func(t *testing.T) {
			t.Run("Then Set stores the value", func(t *testing.T) {
				dst := 9
				monadz.PatchSet(3).Apply(&dst)
				wantDeep(t, dst, 3)
			})
			t.Run("Then Null stores the zero value", func(t *testing.T) {
				dst := 9
				monadz.PatchNull[int]().Apply(&dst)
				wantDeep(t, dst, 0)
			})
			t.Run("Then Missing does nothing", func(t *testing.T) {
				dst := 9
				monadz.PatchMissing[int]().Apply(&dst)
				wantDeep(t, dst, 9)
			})
		})
		t.Run("When ApplyOption is called", func(t *testing.T) {
			t.Run("Then Set stores Some", func(t *testing.T) {
				dst := monadz.Some(9)
				monadz.PatchSet(3).ApplyOption(&dst)
				wantDeep(t, dst, monadz.Some(3))
			})
			t.Run("Then Null stores None", func(t *testing.T) {
				dst := monadz.Some(9)
				monadz.PatchNull[int]().ApplyOption(&dst)
				wantDeep(t, dst.IsNone(), true)
			})
			t.Run("Then Missing does nothing", func(t *testing.T) {
				dst := monadz.Some(9)
				monadz.PatchMissing[int]().ApplyOption(&dst)
				wantDeep(t, dst, monadz.Some(9))
			})
		})
	})
}

func optPatchDecoders() map[string]func([]byte, any) error {
	return map[string]func([]byte, any) error{
		"v1": json.Unmarshal,
		"v2": func(b []byte, v any) error { return jsonv2.Unmarshal(b, v) },
	}
}

func optPatchEncoders() map[string]func(any) ([]byte, error) {
	return map[string]func(any) ([]byte, error){
		"v1": json.Marshal,
		"v2": func(v any) ([]byte, error) { return jsonv2.Marshal(v) },
	}
}

func TestPatchJSON(t *testing.T) {
	for name, decode := range optPatchDecoders() {
		t.Run("Given a PATCH body decoded with encoding/json "+name, func(t *testing.T) {
			t.Run("When a field is missing, a field is null, and a field is set", func(t *testing.T) {
				var r optPatchReq
				err := decode([]byte(`{"name":null,"age":0}`), &r)
				wantNil(t, err)
				t.Run("Then null gives Null and a value gives Set, also for zero", func(t *testing.T) {
					wantDeep(t, r.Name.IsNull(), true)
					wantDeep(t, r.Age.IsSet(), true)
					wantDeep(t, r.Age.Option(), monadz.Some(0))
				})
			})
			t.Run("When the body is empty", func(t *testing.T) {
				var r optPatchReq
				wantNil(t, decode([]byte(`{}`), &r))
				t.Run("Then both fields are Missing", func(t *testing.T) {
					wantDeep(t, r.Name.IsMissing(), true)
					wantDeep(t, r.Age.IsMissing(), true)
				})
			})
			t.Run("When a field has the wrong type", func(t *testing.T) {
				var r optPatchReq
				err := decode([]byte(`{"age":"x"}`), &r)
				t.Run("Then decoding fails", func(t *testing.T) {
					wantNotNil(t, err)
				})
			})
			t.Run("When a truncated null is decoded into a Patch", func(t *testing.T) {
				var p monadz.Patch[int]
				t.Run("Then decoding fails", func(t *testing.T) {
					wantNotNil(t, decode([]byte(`nul`), &p))
				})
			})
		})
	}

	for name, encode := range optPatchEncoders() {
		t.Run("Given Patch values encoded with encoding/json "+name, func(t *testing.T) {
			t.Run("When a request with Set and Missing fields is encoded", func(t *testing.T) {
				b, err := encode(optPatchReq{Age: monadz.PatchSet(0)})
				wantNil(t, err)
				t.Run("Then omitzero drops Missing and keeps Set(0)", func(t *testing.T) {
					wantDeep(t, string(b), `{"age":0}`)
				})
			})
			t.Run("When a request with a Null field is encoded", func(t *testing.T) {
				b, err := encode(optPatchReq{Name: monadz.PatchNull[string]()})
				wantNil(t, err)
				t.Run("Then the Null field writes null", func(t *testing.T) {
					wantDeep(t, string(b), `{"name":null}`)
				})
			})
			t.Run("When a bare Missing Patch is encoded", func(t *testing.T) {
				b, err := encode(monadz.PatchMissing[int]())
				wantNil(t, err)
				t.Run("Then it writes null", func(t *testing.T) {
					wantDeep(t, string(b), "null")
				})
			})
		})
	}

	t.Run("Given Patch values for a round trip", func(t *testing.T) {
		cases := []optPatchReq{
			{Name: monadz.PatchSet("bob"), Age: monadz.PatchSet(3)},
			{Name: monadz.PatchNull[string](), Age: monadz.PatchSet(0)},
			{Name: monadz.PatchSet(""), Age: monadz.PatchNull[int]()},
			{},
		}
		for _, in := range cases {
			t.Run(fmt.Sprintf("When %v is encoded and decoded with v1 and v2", in), func(t *testing.T) {
				b1, err := json.Marshal(in)
				wantNil(t, err)
				var out1 optPatchReq
				wantNil(t, json.Unmarshal(b1, &out1))
				b2, err := jsonv2.Marshal(in)
				wantNil(t, err)
				var out2 optPatchReq
				wantNil(t, jsonv2.Unmarshal(b2, &out2))
				t.Run("Then both come back unchanged", func(t *testing.T) {
					wantDeep(t, out1, in)
					wantDeep(t, out2, in)
					wantDeep(t, string(b1), string(b2))
				})
			})
		}
	})

	t.Run("Given a Set Patch", func(t *testing.T) {
		t.Run("When UnmarshalJSON receives bad input", func(t *testing.T) {
			p := monadz.PatchSet(1)
			err := p.UnmarshalJSON([]byte(`"x"`))
			t.Run("Then it gives an error and keeps the old Patch", func(t *testing.T) {
				wantNotNil(t, err)
				wantDeep(t, p, monadz.PatchSet(1))
			})
		})
		t.Run("When UnmarshalJSON receives null with spaces", func(t *testing.T) {
			p := monadz.PatchSet(1)
			wantNil(t, p.UnmarshalJSON([]byte(" null\n")))
			t.Run("Then it is Null", func(t *testing.T) {
				wantDeep(t, p.IsNull(), true)
			})
		})
		t.Run("When UnmarshalJSON receives a value", func(t *testing.T) {
			p := monadz.PatchNull[int]()
			wantNil(t, p.UnmarshalJSON([]byte("7")))
			t.Run("Then it is Set", func(t *testing.T) {
				wantDeep(t, p, monadz.PatchSet(7))
			})
		})
		t.Run("When MarshalJSON is called directly", func(t *testing.T) {
			t.Run("Then Set writes the value, and Null and Missing write null", func(t *testing.T) {
				b, err := monadz.PatchSet("a").MarshalJSON()
				wantNil(t, err)
				wantDeep(t, string(b), `"a"`)
				for _, p := range []monadz.Patch[string]{monadz.PatchNull[string](), monadz.PatchMissing[string]()} {
					b, err = p.MarshalJSON()
					wantNil(t, err)
					wantDeep(t, string(b), "null")
				}
			})
		})
	})
}

func TestPatchMapMatch(t *testing.T) {
	double := func(n int) int { return n * 2 }
	describe := func(p monadz.Patch[int]) string {
		return p.Match(
			func(n int) string { return fmt.Sprint("set ", n) },
			func() string { return "null" },
			func() string { return "missing" },
		)
	}
	t.Run("Given a Set, a Null, and a Missing Patch", func(t *testing.T) {
		set, null, missing := monadz.PatchSet(5), monadz.PatchNull[int](), monadz.PatchMissing[int]()
		t.Run("When Map applies f", func(t *testing.T) {
			t.Run("Then Set gives Set(f(v)), Null stays Null, and Missing stays Missing", func(t *testing.T) {
				wantDeep(t, set.Map(double), monadz.PatchSet(10))
				wantDeep(t, null.Map(double), monadz.PatchNull[int]())
				wantDeep(t, missing.Map(double), monadz.PatchMissing[int]())
				wantDeep(t, set.Map(strconv.Itoa), monadz.PatchSet("5"))
			})
		})
		t.Run("When Match is called", func(t *testing.T) {
			t.Run("Then it calls the function for the state", func(t *testing.T) {
				wantEq(t, describe(set), "set 5")
				wantEq(t, describe(null), "null")
				wantEq(t, describe(missing), "missing")
			})
		})
	})
}

func TestPatchDecodeError(t *testing.T) {
	for name, decode := range optPatchDecoders() {
		t.Run("Given a field of the wrong JSON type, decoded with "+name, func(t *testing.T) {
			var r optPatchReq
			err := decode([]byte(`{"age":"old"}`), &r)
			t.Run("When decoding fails", func(t *testing.T) {
				t.Run("Then the Patch stays Missing", func(t *testing.T) {
					wantNotNil(t, err)
					wantEq(t, r.Age.IsMissing(), true)
				})
			})
		})
	}
}
