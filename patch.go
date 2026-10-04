package monadz

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
)

// Patch is a field of a JSON PATCH request. It has three states:
//
//	field not in the JSON   -> Missing: do not change the stored value
//	"field": null           -> Null:    clear the stored value
//	"field": <value>        -> Set:     store the value
//
// An Option cannot tell Missing from Null. Patch can. The zero value is
// Missing. Tag the field with `json:",omitzero"` so that a Missing field is
// not written.
type Patch[T any] struct {
	value T
	state patchState
}

type patchState uint8

const (
	patchMissing patchState = iota
	patchNull
	patchSet
)

// PatchSet returns a Patch that sets the value v.
func PatchSet[T any](v T) Patch[T] { return Patch[T]{value: v, state: patchSet} }

// PatchNull returns a Patch that clears the value.
func PatchNull[T any]() Patch[T] { return Patch[T]{state: patchNull} }

// PatchMissing returns a Patch that leaves the value unchanged.
func PatchMissing[T any]() Patch[T] { return Patch[T]{} }

// IsMissing reports whether the field was not in the request.
func (p Patch[T]) IsMissing() bool { return p.state == patchMissing }

// IsNull reports whether the field was null.
func (p Patch[T]) IsNull() bool { return p.state == patchNull }

// IsSet reports whether the field holds a value.
func (p Patch[T]) IsSet() bool { return p.state == patchSet }

// IsZero reports whether the Patch is Missing. It makes omitzero work.
func (p Patch[T]) IsZero() bool { return p.state == patchMissing }

// Value returns the value as an Option: Some for Set, None otherwise.
func (p Patch[T]) Value() Option[T] { return FromTuple(p.value, p.state == patchSet) }

// Apply changes *dst: Set stores the value, Null stores the zero value,
// and Missing does nothing.
func (p Patch[T]) Apply(dst *T) {
	switch p.state {
	case patchSet:
		*dst = p.value
	case patchNull:
		var zero T
		*dst = zero
	}
}

// ApplyOption changes *dst: Set stores Some, Null stores None, and Missing
// does nothing.
func (p Patch[T]) ApplyOption(dst *Option[T]) {
	switch p.state {
	case patchSet:
		*dst = Some(p.value)
	case patchNull:
		*dst = None[T]()
	}
}

// String returns "Set(<value>)", "Null", or "Missing".
func (p Patch[T]) String() string {
	switch p.state {
	case patchSet:
		return fmt.Sprintf("Set(%v)", p.value)
	case patchNull:
		return "Null"
	default:
		return "Missing"
	}
}

// MarshalJSON implements json.Marshaler. Missing and Null both write null;
// use omitzero to drop a Missing field.
func (p Patch[T]) MarshalJSON() ([]byte, error) {
	if p.state != patchSet {
		return []byte("null"), nil
	}
	return json.Marshal(p.value)
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *Patch[T]) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		*p = PatchNull[T]()
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*p = PatchSet(v)
	return nil
}

// MarshalJSONTo implements the encoding/json/v2 MarshalerTo interface.
func (p Patch[T]) MarshalJSONTo(enc *jsontext.Encoder) error {
	if p.state != patchSet {
		return enc.WriteToken(jsontext.Null)
	}
	return jsonv2.MarshalEncode(enc, p.value)
}

// UnmarshalJSONFrom implements the encoding/json/v2 UnmarshalerFrom interface.
func (p *Patch[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() == 'n' {
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		*p = PatchNull[T]()
		return nil
	}
	var v T
	if err := jsonv2.UnmarshalDecode(dec, &v); err != nil {
		return err
	}
	*p = PatchSet(v)
	return nil
}
