package lift_test

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"log/slog"
	"testing"

	"github.com/zkrebbekx/lift"
)

type optRecord struct {
	Name lift.Option[string] `json:"name"`
	Age  lift.Option[int]    `json:"age,omitzero"`
}

type optPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Compile-time checks: Option implements the database/sql interfaces.
var (
	_ sql.Scanner   = (*lift.Option[int])(nil)
	_ driver.Valuer = lift.Option[int]{}
)

func TestOptionJSONv1(t *testing.T) {
	t.Run("Given encoding/json (v1)", func(t *testing.T) {
		t.Run("When Some and None are marshaled", func(t *testing.T) {
			t.Run("Then Some writes the value and None writes null", func(t *testing.T) {
				b, err := json.Marshal(lift.Some(42))
				optNil(t, err)
				optEq(t, string(b), "42")
				b, err = json.Marshal(lift.None[int]())
				optNil(t, err)
				optEq(t, string(b), "null")
				b, err = json.Marshal(lift.Some(optPoint{1, 2}))
				optNil(t, err)
				optEq(t, string(b), `{"x":1,"y":2}`)
			})
		})
		t.Run("When a struct with omitzero is marshaled", func(t *testing.T) {
			t.Run("Then Some(0) is kept", func(t *testing.T) {
				b, err := json.Marshal(optRecord{Name: lift.Some("a"), Age: lift.Some(0)})
				optNil(t, err)
				optEq(t, string(b), `{"name":"a","age":0}`)
			})
			t.Run("Then None is dropped, and a field without omitzero writes null", func(t *testing.T) {
				b, err := json.Marshal(optRecord{})
				optNil(t, err)
				optEq(t, string(b), `{"name":null}`)
			})
		})
		t.Run("When JSON is unmarshaled into a struct", func(t *testing.T) {
			t.Run("Then a value gives Some and null gives None", func(t *testing.T) {
				r := optRecord{Name: lift.Some("old"), Age: lift.Some(9)}
				optNil(t, json.Unmarshal([]byte(`{"name":null,"age":3}`), &r))
				optEq(t, r.Name.IsNone(), true)
				optEq(t, r.Age, lift.Some(3))
			})
			t.Run("Then a missing field keeps the old value", func(t *testing.T) {
				r := optRecord{Age: lift.Some(9)}
				optNil(t, json.Unmarshal([]byte(`{}`), &r))
				optEq(t, r.Age, lift.Some(9))
			})
			t.Run("Then a type mismatch gives an error", func(t *testing.T) {
				var r optRecord
				optNotNil(t, json.Unmarshal([]byte(`{"age":"x"}`), &r))
			})
		})
		t.Run("When UnmarshalJSON is called directly", func(t *testing.T) {
			t.Run("Then null with spaces gives None", func(t *testing.T) {
				o := lift.Some(1)
				optNil(t, o.UnmarshalJSON([]byte("  null ")))
				optEq(t, o.IsNone(), true)
			})
			t.Run("Then bad input gives an error and keeps the old value", func(t *testing.T) {
				o := lift.Some(1)
				optNotNil(t, o.UnmarshalJSON([]byte(`"x"`)))
				optEq(t, o, lift.Some(1))
			})
			t.Run("Then a value gives Some", func(t *testing.T) {
				var o lift.Option[int]
				optNil(t, o.UnmarshalJSON([]byte("5")))
				optEq(t, o, lift.Some(5))
			})
		})
		t.Run("When MarshalJSON is called directly", func(t *testing.T) {
			t.Run("Then Some writes the value and None writes null", func(t *testing.T) {
				b, err := lift.Some(optPoint{1, 2}).MarshalJSON()
				optNil(t, err)
				optEq(t, string(b), `{"x":1,"y":2}`)
				b, err = lift.None[int]().MarshalJSON()
				optNil(t, err)
				optEq(t, string(b), "null")
			})
		})
		t.Run("When values round trip", func(t *testing.T) {
			for _, in := range []lift.Option[optPoint]{lift.Some(optPoint{3, 4}), lift.Some(optPoint{}), lift.None[optPoint]()} {
				t.Run(fmt.Sprintf("Then %v comes back unchanged", in), func(t *testing.T) {
					b, err := json.Marshal(in)
					optNil(t, err)
					var out lift.Option[optPoint]
					optNil(t, json.Unmarshal(b, &out))
					optEq(t, out, in)
				})
			}
		})
	})
}

func TestOptionJSONv2(t *testing.T) {
	t.Run("Given encoding/json/v2", func(t *testing.T) {
		t.Run("When Some and None are marshaled", func(t *testing.T) {
			t.Run("Then Some writes the value and None writes null", func(t *testing.T) {
				b, err := jsonv2.Marshal(lift.Some("hi"))
				optNil(t, err)
				optEq(t, string(b), `"hi"`)
				b, err = jsonv2.Marshal(lift.None[string]())
				optNil(t, err)
				optEq(t, string(b), "null")
			})
		})
		t.Run("When a struct with omitzero is marshaled", func(t *testing.T) {
			t.Run("Then Some(0) is kept and None is dropped", func(t *testing.T) {
				b, err := jsonv2.Marshal(optRecord{Name: lift.Some("a"), Age: lift.Some(0)})
				optNil(t, err)
				optEq(t, string(b), `{"name":"a","age":0}`)
				b, err = jsonv2.Marshal(optRecord{})
				optNil(t, err)
				optEq(t, string(b), `{"name":null}`)
			})
		})
		t.Run("When JSON is unmarshaled into a struct", func(t *testing.T) {
			t.Run("Then a value gives Some and null gives None", func(t *testing.T) {
				r := optRecord{Name: lift.Some("old")}
				optNil(t, jsonv2.Unmarshal([]byte(`{"name":null,"age":3}`), &r))
				optEq(t, r.Name.IsNone(), true)
				optEq(t, r.Age, lift.Some(3))
			})
			t.Run("Then a type mismatch gives an error", func(t *testing.T) {
				var r optRecord
				optNotNil(t, jsonv2.Unmarshal([]byte(`{"age":"x"}`), &r))
			})
			t.Run("Then truncated input gives an error", func(t *testing.T) {
				var o lift.Option[int]
				optNotNil(t, jsonv2.Unmarshal([]byte(`nul`), &o))
			})
		})
		t.Run("When values round trip", func(t *testing.T) {
			for _, in := range []lift.Option[optPoint]{lift.Some(optPoint{3, 4}), lift.Some(optPoint{}), lift.None[optPoint]()} {
				t.Run(fmt.Sprintf("Then %v comes back unchanged", in), func(t *testing.T) {
					b, err := jsonv2.Marshal(in)
					optNil(t, err)
					var out lift.Option[optPoint]
					optNil(t, jsonv2.Unmarshal(b, &out))
					optEq(t, out, in)
				})
			}
		})
	})
}

func TestOptionSQL(t *testing.T) {
	t.Run("Given an Option used with database/sql", func(t *testing.T) {
		t.Run("When Scan receives nil", func(t *testing.T) {
			o := lift.Some(1)
			err := o.Scan(nil)
			t.Run("Then the Option is None", func(t *testing.T) {
				optNil(t, err)
				optEq(t, o.IsNone(), true)
			})
		})
		t.Run("When Scan receives an int64 into Option[int]", func(t *testing.T) {
			var o lift.Option[int]
			err := o.Scan(int64(42))
			t.Run("Then the Option is Some(42)", func(t *testing.T) {
				optNil(t, err)
				optEq(t, o, lift.Some(42))
			})
		})
		t.Run("When Scan receives a string into Option[string]", func(t *testing.T) {
			var o lift.Option[string]
			optNil(t, o.Scan("abc"))
			t.Run("Then the Option is Some(abc)", func(t *testing.T) {
				optEq(t, o, lift.Some("abc"))
			})
		})
		t.Run("When Scan receives []byte", func(t *testing.T) {
			var s lift.Option[string]
			var b lift.Option[[]byte]
			optNil(t, s.Scan([]byte("xyz")))
			optNil(t, b.Scan([]byte("xyz")))
			t.Run("Then Option[string] and Option[[]byte] hold the bytes", func(t *testing.T) {
				optEq(t, s, lift.Some("xyz"))
				optEq(t, b.Must(), []byte("xyz"))
			})
		})
		t.Run("When Scan receives a value of a wrong type", func(t *testing.T) {
			o := lift.Some(7)
			err := o.Scan("not a number")
			t.Run("Then it gives an error and keeps the old value", func(t *testing.T) {
				optNotNil(t, err)
				optEq(t, o, lift.Some(7))
			})
		})
		t.Run("When Value is called", func(t *testing.T) {
			t.Run("Then None gives nil", func(t *testing.T) {
				v, err := lift.None[int]().Value()
				optNil(t, err)
				optNil(t, v)
			})
			t.Run("Then Some gives a driver value", func(t *testing.T) {
				v, err := lift.Some(5).Value()
				optNil(t, err)
				optEq(t, v, int64(5))
				v, err = lift.Some("s").Value()
				optNil(t, err)
				optEq(t, v, "s")
			})
			t.Run("Then Some of an unsupported type gives an error", func(t *testing.T) {
				_, err := lift.Some(optPoint{}).Value()
				optNotNil(t, err)
			})
		})
	})
}

func TestOptionFormatting(t *testing.T) {
	t.Run("Given Some and None", func(t *testing.T) {
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then it gives Some(<value>) or None", func(t *testing.T) {
				optEq(t, lift.Some(3).String(), "Some(3)")
				optEq(t, lift.None[int]().String(), "None")
				optEq(t, fmt.Sprint(lift.Some("a")), "Some(a)")
			})
		})
		t.Run("When IsZero is called", func(t *testing.T) {
			t.Run("Then only None is zero", func(t *testing.T) {
				optEq(t, lift.None[int]().IsZero(), true)
				optEq(t, lift.Some(0).IsZero(), false)
			})
		})
		t.Run("When the Options are logged through a slog JSON handler", func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))
			log.Info("m", "some", lift.Some(5), "none", lift.None[int](), "pt", lift.Some(optPoint{1, 2}))
			var rec map[string]any
			optNil(t, json.Unmarshal(buf.Bytes(), &rec))
			t.Run("Then Some logs the value and None logs null", func(t *testing.T) {
				optEq(t, rec["some"], float64(5))
				v, ok := rec["none"]
				optEq(t, ok, true)
				optNil(t, v)
				optEq(t, rec["pt"], map[string]any{"x": float64(1), "y": float64(2)})
			})
		})
		t.Run("When LogValue is called directly", func(t *testing.T) {
			t.Run("Then it gives the value or nil", func(t *testing.T) {
				optEq(t, lift.Some(5).LogValue().Any(), int64(5))
				optNil(t, lift.None[int]().LogValue().Any())
			})
		})
	})
}
