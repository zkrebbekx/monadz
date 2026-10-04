package scenarios

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zkrebbekx/monadz"
)

// ErrNameRequired is the error for {"name": null}: a tenant must have a name.
var ErrNameRequired = errors.New("name cannot be cleared")

// TenantRecord is the stored tenant settings.
type TenantRecord struct {
	Name         string
	Logo         monadz.Option[string]
	SupportEmail monadz.Option[string]
}

func storedTenant() TenantRecord {
	return TenantRecord{Name: "Acme", Logo: monadz.Some("acme.png"), SupportEmail: monadz.Some("help@acme.example")}
}

// UpdateTenant is the PATCH /tenant request body.
type UpdateTenant struct {
	Name         monadz.Patch[string] `json:"name,omitzero"`
	Logo         monadz.Patch[string] `json:"logo,omitzero"`
	SupportEmail monadz.Patch[string] `json:"support_email,omitzero"`
}

// UpdateTenantOption is the same body with Option fields. It cannot tell
// null from a missing field.
type UpdateTenantOption struct {
	Name         monadz.Option[string] `json:"name"`
	Logo         monadz.Option[string] `json:"logo"`
	SupportEmail monadz.Option[string] `json:"support_email"`
}

// PatchInput is the input of the Patch scenario.
type PatchInput struct {
	Body string `json:"body"`
}

// PatchField is one field of the PATCH, before and after.
type PatchField struct {
	Field       string `json:"field"`
	State       string `json:"state"` // Missing, Null, or Set
	Sent        string `json:"sent"`
	Before      string `json:"before"`
	After       string `json:"after"`
	OptionState string `json:"optionState"` // Some or None
	OptionAfter string `json:"optionAfter"`
	Differs     bool   `json:"differs"`
}

// PatchTrace is the result of the Patch scenario.
type PatchTrace struct {
	Error  string       `json:"error,omitempty"`
	Status int          `json:"status"`
	Fields []PatchField `json:"fields"`
}

func decodeStrict(body string, v any) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(body)))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// RunPatch decodes a PATCH body and applies it to the stored tenant. It also
// applies the same body with Option fields, to show what goes wrong.
func RunPatch(in PatchInput) PatchTrace {
	var req UpdateTenant
	if err := decodeStrict(in.Body, &req); err != nil {
		return PatchTrace{Error: "invalid JSON: " + err.Error(), Status: http.StatusBadRequest, Fields: []PatchField{}}
	}
	var optReq UpdateTenantOption
	_ = decodeStrict(in.Body, &optReq) // the same body decodes in both shapes

	before := storedTenant()

	// The monadz way: three states per field.
	after := before
	var applyErr error
	if req.Name.IsNull() {
		applyErr = ErrNameRequired
	} else {
		req.Name.Apply(&after.Name)
		req.Logo.ApplyOption(&after.Logo)
		req.SupportEmail.ApplyOption(&after.SupportEmail)
	}

	// The Option way: None must mean "keep", because a missing field is
	// None too. So {"logo": null} cannot clear the logo.
	optAfter := before
	if v, ok := optReq.Name.Get(); ok {
		optAfter.Name = v
	}
	if v, ok := optReq.Logo.Get(); ok {
		optAfter.Logo = monadz.Some(v)
	}
	if v, ok := optReq.SupportEmail.Get(); ok {
		optAfter.SupportEmail = monadz.Some(v)
	}

	t := PatchTrace{Status: http.StatusOK}
	if applyErr != nil {
		t.Error, t.Status = applyErr.Error(), http.StatusUnprocessableEntity
		after = before
	}
	t.Fields = []PatchField{
		patchField("name", req.Name, optReq.Name, monadz.Some(before.Name), monadz.Some(after.Name), monadz.Some(optAfter.Name)),
		patchField("logo", req.Logo, optReq.Logo, before.Logo, after.Logo, optAfter.Logo),
		patchField("support_email", req.SupportEmail, optReq.SupportEmail, before.SupportEmail, after.SupportEmail, optAfter.SupportEmail),
	}
	return t
}

func patchField(name string, p monadz.Patch[string], o monadz.Option[string], before, after, optAfter monadz.Option[string]) PatchField {
	f := PatchField{Field: name, State: "Missing", Sent: "(not sent)", OptionState: "None"}
	switch {
	case p.IsNull():
		f.State, f.Sent = "Null", "null"
	case p.IsSet():
		f.State, f.Sent = "Set", quote(p.Option().ValueOrZero())
	}
	if o.IsSome() {
		f.OptionState = "Some"
	}
	f.Before, f.After, f.OptionAfter = showOpt(before), showOpt(after), showOpt(optAfter)
	f.Differs = f.After != f.OptionAfter
	return f
}

func showOpt(o monadz.Option[string]) string {
	return o.MapOr("null", quote)
}
