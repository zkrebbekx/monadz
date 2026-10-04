package monadz

import (
	"encoding/json/jsontext"
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
//
// Use omitzero, not omitempty. Under encoding/json/v2, omitempty drops every
// value that encodes as null, "", [] or {}, including a Null Patch and
// Some("").
type Patch[T any] struct {
	opt     Option[T] // Some for Set, None for Null and Missing
	present bool      // true for Set and Null
}

// PatchSet returns a Patch that sets the value v.
func PatchSet[T any](v T) Patch[T] { return Patch[T]{opt: Some(v), present: true} }

// PatchNull returns a Patch that clears the value.
func PatchNull[T any]() Patch[T] { return Patch[T]{present: true} }

// PatchMissing returns a Patch that leaves the value unchanged.
func PatchMissing[T any]() Patch[T] { return Patch[T]{} }

// IsMissing reports whether the field was not in the request.
func (p Patch[T]) IsMissing() bool { return !p.present }

// IsNull reports whether the field was null.
func (p Patch[T]) IsNull() bool { return p.present && p.opt.IsNone() }

// IsSet reports whether the field holds a value.
func (p Patch[T]) IsSet() bool { return p.opt.IsSome() }

// IsZero reports whether the Patch is Missing. It makes omitzero work.
func (p Patch[T]) IsZero() bool { return !p.present }

// Option returns the value as an Option: Some for Set, None otherwise.
func (p Patch[T]) Option() Option[T] { return p.opt }

// Apply changes *dst: Set stores the value, Null stores the zero value,
// and Missing does nothing.
func (p Patch[T]) Apply(dst *T) {
	if p.present {
		*dst = p.opt.ValueOrZero()
	}
}

// ApplyOption changes *dst: Set stores Some, Null stores None, and Missing
// does nothing.
func (p Patch[T]) ApplyOption(dst *Option[T]) {
	if p.present {
		*dst = p.opt
	}
}

// Map applies f to the value of a Set Patch. A Null Patch stays Null, and a
// Missing Patch stays Missing.
func (p Patch[T]) Map[U any](f func(T) U) Patch[U] {
	return Patch[U]{opt: p.opt.Map(f), present: p.present}
}

// Match calls onSet with the value, or onNull, or onMissing, and returns
// the result.
func (p Patch[T]) Match[U any](onSet func(T) U, onNull func() U, onMissing func() U) U {
	if !p.present {
		return onMissing()
	}
	return p.opt.Match(onSet, onNull)
}

// String returns "Set(<value>)", "Null", or "Missing".
func (p Patch[T]) String() string {
	switch {
	case !p.present:
		return "Missing"
	case p.opt.IsNone():
		return "Null"
	default:
		return fmt.Sprintf("Set(%v)", printable(p.opt.value))
	}
}

// MarshalJSON implements json.Marshaler. Missing and Null both write null;
// use omitzero to drop a Missing field.
//
// encoding/json in Go 1.27 calls MarshalJSONTo first. This method is for
// direct callers.
func (p Patch[T]) MarshalJSON() ([]byte, error) { return p.opt.MarshalJSON() }

// UnmarshalJSON implements json.Unmarshaler: null gives Null, and a value
// gives Set. As with Option, a value that decodes from null gives Null.
//
// encoding/json in Go 1.27 calls UnmarshalJSONFrom first. This method is
// for direct callers.
func (p *Patch[T]) UnmarshalJSON(b []byte) error {
	if err := p.opt.UnmarshalJSON(b); err != nil {
		return err
	}
	p.present = true
	return nil
}

// MarshalJSONTo implements the encoding/json/v2 MarshalerTo interface.
// Missing and Null both write null.
func (p Patch[T]) MarshalJSONTo(enc *jsontext.Encoder) error { return p.opt.MarshalJSONTo(enc) }

// UnmarshalJSONFrom implements the encoding/json/v2 UnmarshalerFrom
// interface: null gives Null, and a value gives Set.
func (p *Patch[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if err := p.opt.UnmarshalJSONFrom(dec); err != nil {
		return err
	}
	p.present = true
	return nil
}
