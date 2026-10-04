package monadz

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
)

// IsZero reports whether the Option is None. Some(0), Some(""), and
// Some(false) are NOT zero, so `json:",omitzero"` drops only None.
//
// Use omitzero, not omitempty. Under encoding/json/v2, omitempty drops every
// value that encodes as null, "", [] or {}, including a Null Patch and
// Some("").
func (o Option[T]) IsZero() bool { return !o.ok }

// MarshalJSON implements json.Marshaler: the value, or null for None.
//
// encoding/json in Go 1.27 calls MarshalJSONTo first. This method is for
// direct callers.
func (o Option[T]) MarshalJSON() ([]byte, error) {
	if !o.ok {
		return []byte("null"), nil
	}
	return json.Marshal(&o.value) // a pointer, so a pointer-receiver MarshalJSON of T runs
}

// UnmarshalJSON implements json.Unmarshaler: null gives None.
// To tell null from a missing field, use Patch.
//
// Some(v) where v encodes as null (a None, or a nil pointer) decodes as
// None. Use Patch to keep the difference. Rust serde behaves the same.
//
// encoding/json in Go 1.27 calls UnmarshalJSONFrom first. This method is
// for direct callers.
func (o *Option[T]) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		*o = None[T]()
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*o = Some(v)
	return nil
}

// MarshalJSONTo implements the encoding/json/v2 MarshalerTo interface.
func (o Option[T]) MarshalJSONTo(enc *jsontext.Encoder) error {
	if !o.ok {
		return enc.WriteToken(jsontext.Null)
	}
	return jsonv2.MarshalEncode(enc, &o.value)
}

// UnmarshalJSONFrom implements the encoding/json/v2 UnmarshalerFrom
// interface: null gives None. As with UnmarshalJSON, Some(v) where v encodes
// as null decodes as None.
func (o *Option[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() == 'n' {
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		*o = None[T]()
		return nil
	}
	var v T
	if err := jsonv2.UnmarshalDecode(dec, &v); err != nil {
		return err
	}
	*o = Some(v)
	return nil
}

// Scan implements sql.Scanner: a NULL column gives None.
func (o *Option[T]) Scan(src any) error {
	if src == nil {
		*o = None[T]()
		return nil
	}
	var n sql.Null[T]
	if err := n.Scan(src); err != nil {
		return err
	}
	*o = Some(n.V)
	return nil
}

// Value implements driver.Valuer: None gives NULL.
//
// Value accepts only the value types of database/sql's
// DefaultParameterConverter, or a T that implements driver.Valuer. For any
// other T, Value returns an error.
func (o Option[T]) Value() (driver.Value, error) {
	if !o.ok {
		return nil, nil
	}
	return driver.DefaultParameterConverter.ConvertValue(o.value)
}
