package lift_test

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"testing"

	"github.com/zkrebbekx/lift"
)

type optPatchReq struct {
	Name lift.Patch[string] `json:"name,omitzero"`
	Age  lift.Patch[int]    `json:"age,omitzero"`
}

func TestPatchStates(t *testing.T) {
	t.Run("Given the three Patch constructors", func(t *testing.T) {
		set, null, missing := lift.PatchSet(5), lift.PatchNull[int](), lift.PatchMissing[int]()
		t.Run("When the states are queried", func(t *testing.T) {
			t.Run("Then each Patch reports only its own state", func(t *testing.T) {
				optEq(t, []bool{set.IsSet(), set.IsNull(), set.IsMissing()}, []bool{true, false, false})
				optEq(t, []bool{null.IsSet(), null.IsNull(), null.IsMissing()}, []bool{false, true, false})
				optEq(t, []bool{missing.IsSet(), missing.IsNull(), missing.IsMissing()}, []bool{false, false, true})
			})
			t.Run("Then only Missing is zero", func(t *testing.T) {
				optEq(t, missing.IsZero(), true)
				optEq(t, null.IsZero(), false)
				optEq(t, set.IsZero(), false)
				optEq(t, lift.PatchSet(0).IsZero(), false)
			})
			t.Run("Then the zero value of Patch is Missing", func(t *testing.T) {
				var p lift.Patch[int]
				optEq(t, p.IsMissing(), true)
				optEq(t, p, missing)
			})
		})
		t.Run("When Value is called", func(t *testing.T) {
			t.Run("Then Set gives Some and the others give None", func(t *testing.T) {
				optEq(t, set.Value(), lift.Some(5))
				optEq(t, lift.PatchSet(0).Value(), lift.Some(0))
				optEq(t, null.Value().IsNone(), true)
				optEq(t, missing.Value().IsNone(), true)
			})
		})
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then it gives Set(<value>), Null, or Missing", func(t *testing.T) {
				optEq(t, set.String(), "Set(5)")
				optEq(t, null.String(), "Null")
				optEq(t, fmt.Sprint(missing), "Missing")
			})
		})
	})
}

func TestPatchApply(t *testing.T) {
	t.Run("Given a stored value of 9", func(t *testing.T) {
		t.Run("When Apply is called", func(t *testing.T) {
			t.Run("Then Set stores the value", func(t *testing.T) {
				dst := 9
				lift.PatchSet(3).Apply(&dst)
				optEq(t, dst, 3)
			})
			t.Run("Then Null stores the zero value", func(t *testing.T) {
				dst := 9
				lift.PatchNull[int]().Apply(&dst)
				optEq(t, dst, 0)
			})
			t.Run("Then Missing does nothing", func(t *testing.T) {
				dst := 9
				lift.PatchMissing[int]().Apply(&dst)
				optEq(t, dst, 9)
			})
		})
		t.Run("When ApplyOption is called", func(t *testing.T) {
			t.Run("Then Set stores Some", func(t *testing.T) {
				dst := lift.Some(9)
				lift.PatchSet(3).ApplyOption(&dst)
				optEq(t, dst, lift.Some(3))
			})
			t.Run("Then Null stores None", func(t *testing.T) {
				dst := lift.Some(9)
				lift.PatchNull[int]().ApplyOption(&dst)
				optEq(t, dst.IsNone(), true)
			})
			t.Run("Then Missing does nothing", func(t *testing.T) {
				dst := lift.Some(9)
				lift.PatchMissing[int]().ApplyOption(&dst)
				optEq(t, dst, lift.Some(9))
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
				optNil(t, err)
				t.Run("Then null gives Null and a value gives Set, also for zero", func(t *testing.T) {
					optEq(t, r.Name.IsNull(), true)
					optEq(t, r.Age.IsSet(), true)
					optEq(t, r.Age.Value(), lift.Some(0))
				})
			})
			t.Run("When the body is empty", func(t *testing.T) {
				var r optPatchReq
				optNil(t, decode([]byte(`{}`), &r))
				t.Run("Then both fields are Missing", func(t *testing.T) {
					optEq(t, r.Name.IsMissing(), true)
					optEq(t, r.Age.IsMissing(), true)
				})
			})
			t.Run("When a field has the wrong type", func(t *testing.T) {
				var r optPatchReq
				err := decode([]byte(`{"age":"x"}`), &r)
				t.Run("Then decoding fails", func(t *testing.T) {
					optNotNil(t, err)
				})
			})
			t.Run("When a truncated null is decoded into a Patch", func(t *testing.T) {
				var p lift.Patch[int]
				t.Run("Then decoding fails", func(t *testing.T) {
					optNotNil(t, decode([]byte(`nul`), &p))
				})
			})
		})
	}

	for name, encode := range optPatchEncoders() {
		t.Run("Given Patch values encoded with encoding/json "+name, func(t *testing.T) {
			t.Run("When a request with Set and Missing fields is encoded", func(t *testing.T) {
				b, err := encode(optPatchReq{Age: lift.PatchSet(0)})
				optNil(t, err)
				t.Run("Then omitzero drops Missing and keeps Set(0)", func(t *testing.T) {
					optEq(t, string(b), `{"age":0}`)
				})
			})
			t.Run("When a request with a Null field is encoded", func(t *testing.T) {
				b, err := encode(optPatchReq{Name: lift.PatchNull[string]()})
				optNil(t, err)
				t.Run("Then the Null field writes null", func(t *testing.T) {
					optEq(t, string(b), `{"name":null}`)
				})
			})
			t.Run("When a bare Missing Patch is encoded", func(t *testing.T) {
				b, err := encode(lift.PatchMissing[int]())
				optNil(t, err)
				t.Run("Then it writes null", func(t *testing.T) {
					optEq(t, string(b), "null")
				})
			})
		})
	}

	t.Run("Given Patch values for a round trip", func(t *testing.T) {
		cases := []optPatchReq{
			{Name: lift.PatchSet("bob"), Age: lift.PatchSet(3)},
			{Name: lift.PatchNull[string](), Age: lift.PatchSet(0)},
			{Name: lift.PatchSet(""), Age: lift.PatchNull[int]()},
			{},
		}
		for _, in := range cases {
			t.Run(fmt.Sprintf("When %v is encoded and decoded with v1 and v2", in), func(t *testing.T) {
				b1, err := json.Marshal(in)
				optNil(t, err)
				var out1 optPatchReq
				optNil(t, json.Unmarshal(b1, &out1))
				b2, err := jsonv2.Marshal(in)
				optNil(t, err)
				var out2 optPatchReq
				optNil(t, jsonv2.Unmarshal(b2, &out2))
				t.Run("Then both come back unchanged", func(t *testing.T) {
					optEq(t, out1, in)
					optEq(t, out2, in)
					optEq(t, string(b1), string(b2))
				})
			})
		}
	})

	t.Run("Given a Set Patch", func(t *testing.T) {
		t.Run("When UnmarshalJSON receives bad input", func(t *testing.T) {
			p := lift.PatchSet(1)
			err := p.UnmarshalJSON([]byte(`"x"`))
			t.Run("Then it gives an error and keeps the old Patch", func(t *testing.T) {
				optNotNil(t, err)
				optEq(t, p, lift.PatchSet(1))
			})
		})
		t.Run("When UnmarshalJSON receives null with spaces", func(t *testing.T) {
			p := lift.PatchSet(1)
			optNil(t, p.UnmarshalJSON([]byte(" null\n")))
			t.Run("Then it is Null", func(t *testing.T) {
				optEq(t, p.IsNull(), true)
			})
		})
		t.Run("When UnmarshalJSON receives a value", func(t *testing.T) {
			p := lift.PatchNull[int]()
			optNil(t, p.UnmarshalJSON([]byte("7")))
			t.Run("Then it is Set", func(t *testing.T) {
				optEq(t, p, lift.PatchSet(7))
			})
		})
		t.Run("When MarshalJSON is called directly", func(t *testing.T) {
			t.Run("Then Set writes the value, and Null and Missing write null", func(t *testing.T) {
				b, err := lift.PatchSet("a").MarshalJSON()
				optNil(t, err)
				optEq(t, string(b), `"a"`)
				for _, p := range []lift.Patch[string]{lift.PatchNull[string](), lift.PatchMissing[string]()} {
					b, err = p.MarshalJSON()
					optNil(t, err)
					optEq(t, string(b), "null")
				}
			})
		})
	})
}
