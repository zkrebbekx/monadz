package monadz_test

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"log/slog"
	"math/big"
	"testing"

	"github.com/zkrebbekx/monadz"
)

type optRecord struct {
	Name monadz.Option[string] `json:"name"`
	Age  monadz.Option[int]    `json:"age,omitzero"`
}

type optPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Compile-time checks: Option implements the database/sql interfaces.
var (
	_ sql.Scanner   = (*monadz.Option[int])(nil)
	_ driver.Valuer = monadz.Option[int]{}
)

func TestOptionJSONv1(t *testing.T) {
	t.Run("Given encoding/json (v1)", func(t *testing.T) {
		t.Run("When Some and None are marshaled", func(t *testing.T) {
			t.Run("Then Some writes the value and None writes null", func(t *testing.T) {
				b, err := json.Marshal(monadz.Some(42))
				wantNil(t, err)
				wantDeep(t, string(b), "42")
				b, err = json.Marshal(monadz.None[int]())
				wantNil(t, err)
				wantDeep(t, string(b), "null")
				b, err = json.Marshal(monadz.Some(optPoint{1, 2}))
				wantNil(t, err)
				wantDeep(t, string(b), `{"x":1,"y":2}`)
			})
		})
		t.Run("When a struct with omitzero is marshaled", func(t *testing.T) {
			t.Run("Then Some(0) is kept", func(t *testing.T) {
				b, err := json.Marshal(optRecord{Name: monadz.Some("a"), Age: monadz.Some(0)})
				wantNil(t, err)
				wantDeep(t, string(b), `{"name":"a","age":0}`)
			})
			t.Run("Then None is dropped, and a field without omitzero writes null", func(t *testing.T) {
				b, err := json.Marshal(optRecord{})
				wantNil(t, err)
				wantDeep(t, string(b), `{"name":null}`)
			})
		})
		t.Run("When JSON is unmarshaled into a struct", func(t *testing.T) {
			t.Run("Then a value gives Some and null gives None", func(t *testing.T) {
				r := optRecord{Name: monadz.Some("old"), Age: monadz.Some(9)}
				wantNil(t, json.Unmarshal([]byte(`{"name":null,"age":3}`), &r))
				wantDeep(t, r.Name.IsNone(), true)
				wantDeep(t, r.Age, monadz.Some(3))
			})
			t.Run("Then a missing field keeps the old value", func(t *testing.T) {
				r := optRecord{Age: monadz.Some(9)}
				wantNil(t, json.Unmarshal([]byte(`{}`), &r))
				wantDeep(t, r.Age, monadz.Some(9))
			})
			t.Run("Then a type mismatch gives an error", func(t *testing.T) {
				var r optRecord
				wantNotNil(t, json.Unmarshal([]byte(`{"age":"x"}`), &r))
			})
		})
		t.Run("When UnmarshalJSON is called directly", func(t *testing.T) {
			t.Run("Then null with spaces gives None", func(t *testing.T) {
				o := monadz.Some(1)
				wantNil(t, o.UnmarshalJSON([]byte("  null ")))
				wantDeep(t, o.IsNone(), true)
			})
			t.Run("Then bad input gives an error and keeps the old value", func(t *testing.T) {
				o := monadz.Some(1)
				wantNotNil(t, o.UnmarshalJSON([]byte(`"x"`)))
				wantDeep(t, o, monadz.Some(1))
			})
			t.Run("Then a value gives Some", func(t *testing.T) {
				var o monadz.Option[int]
				wantNil(t, o.UnmarshalJSON([]byte("5")))
				wantDeep(t, o, monadz.Some(5))
			})
		})
		t.Run("When MarshalJSON is called directly", func(t *testing.T) {
			t.Run("Then Some writes the value and None writes null", func(t *testing.T) {
				b, err := monadz.Some(optPoint{1, 2}).MarshalJSON()
				wantNil(t, err)
				wantDeep(t, string(b), `{"x":1,"y":2}`)
				b, err = monadz.None[int]().MarshalJSON()
				wantNil(t, err)
				wantDeep(t, string(b), "null")
			})
		})
		t.Run("When values round trip", func(t *testing.T) {
			for _, in := range []monadz.Option[optPoint]{monadz.Some(optPoint{3, 4}), monadz.Some(optPoint{}), monadz.None[optPoint]()} {
				t.Run(fmt.Sprintf("Then %v comes back unchanged", in), func(t *testing.T) {
					b, err := json.Marshal(in)
					wantNil(t, err)
					var out monadz.Option[optPoint]
					wantNil(t, json.Unmarshal(b, &out))
					wantDeep(t, out, in)
				})
			}
		})
	})
}

func TestOptionJSONv2(t *testing.T) {
	t.Run("Given encoding/json/v2", func(t *testing.T) {
		t.Run("When Some and None are marshaled", func(t *testing.T) {
			t.Run("Then Some writes the value and None writes null", func(t *testing.T) {
				b, err := jsonv2.Marshal(monadz.Some("hi"))
				wantNil(t, err)
				wantDeep(t, string(b), `"hi"`)
				b, err = jsonv2.Marshal(monadz.None[string]())
				wantNil(t, err)
				wantDeep(t, string(b), "null")
			})
		})
		t.Run("When a struct with omitzero is marshaled", func(t *testing.T) {
			t.Run("Then Some(0) is kept and None is dropped", func(t *testing.T) {
				b, err := jsonv2.Marshal(optRecord{Name: monadz.Some("a"), Age: monadz.Some(0)})
				wantNil(t, err)
				wantDeep(t, string(b), `{"name":"a","age":0}`)
				b, err = jsonv2.Marshal(optRecord{})
				wantNil(t, err)
				wantDeep(t, string(b), `{"name":null}`)
			})
		})
		t.Run("When JSON is unmarshaled into a struct", func(t *testing.T) {
			t.Run("Then a value gives Some and null gives None", func(t *testing.T) {
				r := optRecord{Name: monadz.Some("old")}
				wantNil(t, jsonv2.Unmarshal([]byte(`{"name":null,"age":3}`), &r))
				wantDeep(t, r.Name.IsNone(), true)
				wantDeep(t, r.Age, monadz.Some(3))
			})
			t.Run("Then a type mismatch gives an error", func(t *testing.T) {
				var r optRecord
				wantNotNil(t, jsonv2.Unmarshal([]byte(`{"age":"x"}`), &r))
			})
			t.Run("Then truncated input gives an error", func(t *testing.T) {
				var o monadz.Option[int]
				wantNotNil(t, jsonv2.Unmarshal([]byte(`nul`), &o))
			})
		})
		t.Run("When values round trip", func(t *testing.T) {
			for _, in := range []monadz.Option[optPoint]{monadz.Some(optPoint{3, 4}), monadz.Some(optPoint{}), monadz.None[optPoint]()} {
				t.Run(fmt.Sprintf("Then %v comes back unchanged", in), func(t *testing.T) {
					b, err := jsonv2.Marshal(in)
					wantNil(t, err)
					var out monadz.Option[optPoint]
					wantNil(t, jsonv2.Unmarshal(b, &out))
					wantDeep(t, out, in)
				})
			}
		})
	})
}

func TestOptionSQL(t *testing.T) {
	t.Run("Given an Option used with database/sql", func(t *testing.T) {
		t.Run("When Scan receives nil", func(t *testing.T) {
			o := monadz.Some(1)
			err := o.Scan(nil)
			t.Run("Then the Option is None", func(t *testing.T) {
				wantNil(t, err)
				wantDeep(t, o.IsNone(), true)
			})
		})
		t.Run("When Scan receives an int64 into Option[int]", func(t *testing.T) {
			var o monadz.Option[int]
			err := o.Scan(int64(42))
			t.Run("Then the Option is Some(42)", func(t *testing.T) {
				wantNil(t, err)
				wantDeep(t, o, monadz.Some(42))
			})
		})
		t.Run("When Scan receives a string into Option[string]", func(t *testing.T) {
			var o monadz.Option[string]
			wantNil(t, o.Scan("abc"))
			t.Run("Then the Option is Some(abc)", func(t *testing.T) {
				wantDeep(t, o, monadz.Some("abc"))
			})
		})
		t.Run("When Scan receives []byte", func(t *testing.T) {
			var s monadz.Option[string]
			var b monadz.Option[[]byte]
			wantNil(t, s.Scan([]byte("xyz")))
			wantNil(t, b.Scan([]byte("xyz")))
			t.Run("Then Option[string] and Option[[]byte] hold the bytes", func(t *testing.T) {
				wantDeep(t, s, monadz.Some("xyz"))
				wantDeep(t, b.Must(), []byte("xyz"))
			})
		})
		t.Run("When Scan receives a value of a wrong type", func(t *testing.T) {
			o := monadz.Some(7)
			err := o.Scan("not a number")
			t.Run("Then it gives an error and keeps the old value", func(t *testing.T) {
				wantNotNil(t, err)
				wantDeep(t, o, monadz.Some(7))
			})
		})
		t.Run("When Value is called", func(t *testing.T) {
			t.Run("Then None gives nil", func(t *testing.T) {
				v, err := monadz.None[int]().Value()
				wantNil(t, err)
				wantNil(t, v)
			})
			t.Run("Then Some gives a driver value", func(t *testing.T) {
				v, err := monadz.Some(5).Value()
				wantNil(t, err)
				wantDeep(t, v, int64(5))
				v, err = monadz.Some("s").Value()
				wantNil(t, err)
				wantDeep(t, v, "s")
			})
			t.Run("Then Some of an unsupported type gives an error", func(t *testing.T) {
				_, err := monadz.Some(optPoint{}).Value()
				wantNotNil(t, err)
			})
		})
	})
}

func TestOptionFormatting(t *testing.T) {
	t.Run("Given Some and None", func(t *testing.T) {
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then it gives Some(<value>) or None", func(t *testing.T) {
				wantDeep(t, monadz.Some(3).String(), "Some(3)")
				wantDeep(t, monadz.None[int]().String(), "None")
				wantDeep(t, fmt.Sprint(monadz.Some("a")), "Some(a)")
			})
		})
		t.Run("When IsZero is called", func(t *testing.T) {
			t.Run("Then only None is zero", func(t *testing.T) {
				wantDeep(t, monadz.None[int]().IsZero(), true)
				wantDeep(t, monadz.Some(0).IsZero(), false)
			})
		})
		t.Run("When the Options are logged through a slog JSON handler", func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))
			log.Info("m", "some", monadz.Some(5), "none", monadz.None[int](), "pt", monadz.Some(optPoint{1, 2}))
			var rec map[string]any
			wantNil(t, json.Unmarshal(buf.Bytes(), &rec))
			t.Run("Then Some logs the value and None logs null", func(t *testing.T) {
				wantDeep(t, rec["some"], float64(5))
				v, ok := rec["none"]
				wantDeep(t, ok, true)
				wantNil(t, v)
				wantDeep(t, rec["pt"], map[string]any{"x": float64(1), "y": float64(2)})
			})
		})
		t.Run("When LogValue is called directly", func(t *testing.T) {
			t.Run("Then it gives the value or nil", func(t *testing.T) {
				wantDeep(t, monadz.Some(5).LogValue().Any(), int64(5))
				wantNil(t, monadz.None[int]().LogValue().Any())
			})
		})
	})
}

func TestPointerReceiverMethods(t *testing.T) {
	t.Run("Given a big.Int, whose String and MarshalJSON have pointer receivers", func(t *testing.T) {
		n := *big.NewInt(5)
		type rec struct {
			A big.Int
			B monadz.Option[big.Int]
			C monadz.Patch[big.Int]
		}
		r := rec{A: n, B: monadz.Some(n), C: monadz.PatchSet(n)}
		t.Run("When encoding/json (v1) encodes a struct with an Option and a Patch of it", func(t *testing.T) {
			got, err := json.Marshal(&r)
			t.Run("Then the Option and the Patch use the pointer-receiver MarshalJSON", func(t *testing.T) {
				wantNil(t, err)
				wantEq(t, string(got), `{"A":5,"B":5,"C":5}`)
			})
		})
		t.Run("When encoding/json/v2 encodes it", func(t *testing.T) {
			got, err := jsonv2.Marshal(&r)
			t.Run("Then the Option and the Patch use the pointer-receiver MarshalJSON", func(t *testing.T) {
				wantNil(t, err)
				wantEq(t, string(got), `{"A":5,"B":5,"C":5}`)
			})
		})
		t.Run("When the MarshalJSON methods are called directly", func(t *testing.T) {
			o, errO := monadz.Some(n).MarshalJSON()
			p, errP := monadz.PatchSet(n).MarshalJSON()
			t.Run("Then they write 5", func(t *testing.T) {
				wantNil(t, errO)
				wantNil(t, errP)
				wantEq(t, string(o), "5")
				wantEq(t, string(p), "5")
			})
		})
		t.Run("When String is called on each type that holds it", func(t *testing.T) {
			t.Run("Then String uses the pointer-receiver String of big.Int", func(t *testing.T) {
				wantEq(t, monadz.Some(n).String(), "Some(5)")
				wantEq(t, monadz.Ok(n).String(), "Ok(5)")
				wantEq(t, monadz.Right[string](n).String(), "Right(5)")
				wantEq(t, monadz.Left[big.Int, int](n).String(), "Left(5)")
				wantEq(t, monadz.Valid(n).String(), "Valid(5)")
				wantEq(t, monadz.PatchSet(n).String(), "Set(5)")
			})
		})
		t.Run("When slog logs an Option and a Result of it as JSON", func(t *testing.T) {
			var buf bytes.Buffer
			slog.New(slog.NewJSONHandler(&buf, nil)).Info("m", "o", monadz.Some(n), "r", monadz.Ok(n))
			t.Run("Then the log holds the number 5", func(t *testing.T) {
				wantContains(t, buf.String(), `"o":5`)
				wantContains(t, buf.String(), `"r":5`)
			})
		})
	})

	t.Run("Given a plain struct with no methods", func(t *testing.T) {
		p := optPoint{X: 1, Y: 2}
		t.Run("When String is called", func(t *testing.T) {
			t.Run("Then the struct prints as {...}, not &{...}", func(t *testing.T) {
				wantEq(t, monadz.Some(p).String(), "Some({1 2})")
				wantEq(t, monadz.Ok(p).String(), "Ok({1 2})")
				wantEq(t, monadz.Valid(p).String(), "Valid({1 2})")
			})
		})
	})
}

func TestOptionJSONNullInside(t *testing.T) {
	t.Run("Given Some(None), whose value encodes as null", func(t *testing.T) {
		in := monadz.Some(monadz.None[int]())
		t.Run("When it is encoded and decoded", func(t *testing.T) {
			b, err := json.Marshal(in)
			var out monadz.Option[monadz.Option[int]]
			uerr := json.Unmarshal(b, &out)
			t.Run("Then it decodes as None, as the doc says", func(t *testing.T) {
				wantNil(t, err)
				wantNil(t, uerr)
				wantEq(t, string(b), "null")
				wantDeep(t, out, monadz.None[monadz.Option[int]]())
			})
		})
	})
}
