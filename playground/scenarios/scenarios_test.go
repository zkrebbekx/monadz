package scenarios

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func tracks(steps []Step) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Track
	}
	return out
}

func wantTracks(t *testing.T, steps []Step, want ...string) {
	t.Helper()
	if got := tracks(steps); !slices.Equal(got, want) {
		t.Fatalf("tracks = %v, want %v", got, want)
	}
}

func TestOption(t *testing.T) {
	t.Run("Given the host of an active tenant with branding", func(t *testing.T) {
		t.Run("When the host has upper case, www, and a port", func(t *testing.T) {
			tr := RunOption(OptionInput{Host: " https://www.ACME.app.example:443/login "})
			t.Run("Then every step is Some and the tenant branding is used", func(t *testing.T) {
				wantTracks(t, tr.Steps, TrackSome, TrackSome, TrackSome, TrackSome)
				if tr.UsedDefault || tr.Branding.Color != "#00ADD8" {
					t.Fatalf("branding = %+v, usedDefault = %v", tr.Branding, tr.UsedDefault)
				}
				if len(tr.Directory) != 3 {
					t.Fatalf("directory has %d rows", len(tr.Directory))
				}
			})
		})
	})
	t.Run("Given a host that no tenant owns", func(t *testing.T) {
		tr := RunOption(OptionInput{Host: "contoso.app.example"})
		t.Run("Then the lookup is None, the rest is skipped, and the default is used", func(t *testing.T) {
			wantTracks(t, tr.Steps, TrackSome, TrackNone, TrackSkipped, TrackSkipped)
			if !tr.UsedDefault || tr.Branding != DefaultBranding {
				t.Fatalf("branding = %+v", tr.Branding)
			}
		})
	})
	t.Run("Given the host of a suspended tenant", func(t *testing.T) {
		tr := RunOption(OptionInput{Host: "globex.app.example"})
		t.Run("Then Filter turns the tenant into None", func(t *testing.T) {
			wantTracks(t, tr.Steps, TrackSome, TrackSome, TrackNone, TrackSkipped)
			if !strings.Contains(tr.Steps[2].Note, "suspended") {
				t.Fatalf("note = %q", tr.Steps[2].Note)
			}
		})
	})
	t.Run("Given an active tenant with no branding", func(t *testing.T) {
		tr := RunOption(OptionInput{Host: "initech.app.example"})
		t.Run("Then only the last step is None", func(t *testing.T) {
			wantTracks(t, tr.Steps, TrackSome, TrackSome, TrackSome, TrackNone)
		})
	})
	t.Run("Given an empty host", func(t *testing.T) {
		tr := RunOption(OptionInput{Host: "   "})
		t.Run("Then the first step is None", func(t *testing.T) {
			wantTracks(t, tr.Steps, TrackNone, TrackSkipped, TrackSkipped, TrackSkipped)
		})
	})
}

var goodSignup = SignupInput{Email: "ada@acme.example", Password: "correct1horse", Company: "Acme"}

func TestResult(t *testing.T) {
	t.Run("Given a valid signup", func(t *testing.T) {
		tr := RunResult(goodSignup)
		t.Run("Then every step is Ok and the status is 201", func(t *testing.T) {
			wantTracks(t, tr.Steps, TrackOk, TrackOk, TrackOk, TrackOk)
			if tr.Status != 201 || !strings.Contains(tr.Body, "acct_") {
				t.Fatalf("status = %d, body = %s", tr.Status, tr.Body)
			}
		})
	})
	t.Run("Given a signup with a bad email", func(t *testing.T) {
		in := goodSignup
		in.Email = "ada"
		tr := RunResult(in)
		t.Run("Then validate fails, the rest is skipped, and the status is 422", func(t *testing.T) {
			wantTracks(t, tr.Steps, TrackErr, TrackSkipped, TrackSkipped, TrackSkipped)
			if tr.Status != 422 {
				t.Fatalf("status = %d", tr.Status)
			}
		})
	})
	t.Run("Given an email that is already taken", func(t *testing.T) {
		in := goodSignup
		in.EmailTaken = true
		tr := RunResult(in)
		t.Run("Then step 2 fails with 409", func(t *testing.T) {
			wantTracks(t, tr.Steps, TrackOk, TrackErr, TrackSkipped, TrackSkipped)
			if tr.Status != 409 {
				t.Fatalf("status = %d", tr.Status)
			}
		})
	})
	t.Run("Given a database that is down", func(t *testing.T) {
		in := goodSignup
		in.DBDown = true
		tr := RunResult(in)
		t.Run("Then the insert fails with 503", func(t *testing.T) {
			wantTracks(t, tr.Steps, TrackOk, TrackOk, TrackOk, TrackErr)
			if tr.Status != 503 {
				t.Fatalf("status = %d", tr.Status)
			}
		})
	})
}

func TestValidation(t *testing.T) {
	t.Run("Given a form with three bad fields", func(t *testing.T) {
		tr := RunValidation(SignupInput{Email: "ada", Password: "abc", Company: ""})
		t.Run("Then Validation reports all four errors and Result reports one", func(t *testing.T) {
			if tr.Valid || len(tr.Errors) != 4 {
				t.Fatalf("errors = %+v", tr.Errors)
			}
			fields := []string{tr.Errors[0].Field, tr.Errors[1].Field, tr.Errors[2].Field, tr.Errors[3].Field}
			if !slices.Equal(fields, []string{"email", "password", "password", "company"}) {
				t.Fatalf("fields = %v", fields)
			}
			if len(tr.ResultErrors) != 1 || tr.ResultErrors[0].Field != "email" {
				t.Fatalf("result errors = %+v", tr.ResultErrors)
			}
			if tr.Status != 422 {
				t.Fatalf("status = %d", tr.Status)
			}
		})
	})
	t.Run("Given a valid form", func(t *testing.T) {
		tr := RunValidation(goodSignup)
		t.Run("Then there are no errors", func(t *testing.T) {
			if !tr.Valid || len(tr.Errors) != 0 || len(tr.ResultErrors) != 0 {
				t.Fatalf("trace = %+v", tr)
			}
		})
	})
}

func TestEither(t *testing.T) {
	t.Run("Given an enterprise tenant that asks for SSO", func(t *testing.T) {
		tr := RunEither(EitherInput{Plan: "enterprise", Feature: "sso", Seats: 40})
		t.Run("Then the answer is Right with status 200", func(t *testing.T) {
			wantTracks(t, tr.Steps, TrackRight, TrackRight, TrackRight)
			if tr.Side != TrackRight || tr.Status != 200 || tr.Grant == nil {
				t.Fatalf("trace = %+v", tr)
			}
		})
	})
	t.Run("Given a team tenant that asks for SSO", func(t *testing.T) {
		tr := RunEither(EitherInput{Plan: "team", Feature: "sso", Seats: 10})
		t.Run("Then the answer is a paywall with an upgrade to enterprise", func(t *testing.T) {
			wantTracks(t, tr.Steps, TrackLeft, TrackSkipped, TrackSkipped)
			if tr.Status != 402 || tr.Paywall == nil || tr.Paywall.UpgradeTo != "enterprise" {
				t.Fatalf("trace = %+v", tr)
			}
		})
	})
	t.Run("Given a free tenant with too many seats", func(t *testing.T) {
		tr := RunEither(EitherInput{Plan: "free", Feature: "api_access", Seats: 10})
		t.Run("Then the seat check gives the paywall", func(t *testing.T) {
			wantTracks(t, tr.Steps, TrackRight, TrackLeft, TrackSkipped)
			if tr.Paywall == nil || tr.Paywall.UpgradeTo != "team" {
				t.Fatalf("trace = %+v", tr)
			}
		})
	})
}

func field(t *testing.T, tr PatchTrace, name string) PatchField {
	t.Helper()
	for _, f := range tr.Fields {
		if f.Field == name {
			return f
		}
	}
	t.Fatalf("no field %q", name)
	return PatchField{}
}

func TestPatch(t *testing.T) {
	t.Run("Given a body that sets the name and clears the logo", func(t *testing.T) {
		tr := RunPatch(PatchInput{Body: `{"name":"Acme Corp","logo":null}`})
		t.Run("Then each field has the right state", func(t *testing.T) {
			if tr.Error != "" || tr.Status != 200 {
				t.Fatalf("trace = %+v", tr)
			}
			name, logo, email := field(t, tr, "name"), field(t, tr, "logo"), field(t, tr, "support_email")
			if name.State != "Set" || name.After != `"Acme Corp"` {
				t.Fatalf("name = %+v", name)
			}
			if logo.State != "Null" || logo.After != "null" {
				t.Fatalf("logo = %+v", logo)
			}
			if email.State != "Missing" || email.After != email.Before {
				t.Fatalf("email = %+v", email)
			}
		})
		t.Run("Then the Option version keeps the logo by mistake", func(t *testing.T) {
			logo := field(t, tr, "logo")
			if logo.OptionState != "None" || logo.OptionAfter != `"acme.png"` || !logo.Differs {
				t.Fatalf("logo = %+v", logo)
			}
		})
	})
	t.Run("Given a body that clears the name", func(t *testing.T) {
		tr := RunPatch(PatchInput{Body: `{"name":null}`})
		t.Run("Then the request fails with 422 and nothing changes", func(t *testing.T) {
			if tr.Status != 422 || field(t, tr, "name").After != `"Acme"` {
				t.Fatalf("trace = %+v", tr)
			}
		})
	})
	t.Run("Given a body that is not JSON", func(t *testing.T) {
		tr := RunPatch(PatchInput{Body: `{"name":`})
		t.Run("Then the status is 400", func(t *testing.T) {
			if tr.Status != 400 || tr.Error == "" {
				t.Fatalf("trace = %+v", tr)
			}
		})
	})
	t.Run("Given a body with an unknown field", func(t *testing.T) {
		tr := RunPatch(PatchInput{Body: `{"plan":"team"}`})
		t.Run("Then the status is 400", func(t *testing.T) {
			if tr.Status != 400 {
				t.Fatalf("trace = %+v", tr)
			}
		})
	})
}

func TestDo(t *testing.T) {
	t.Run("Given no failure and no bug", func(t *testing.T) {
		for _, mode := range []string{"do", "dorecover"} {
			tr := RunDo(DoInput{Mode: mode})
			t.Run("Then "+mode+" returns Ok", func(t *testing.T) {
				if tr.Outcome != "ok" {
					t.Fatalf("trace = %+v", tr)
				}
				wantTracks(t, tr.Steps, TrackOk, TrackOk, TrackOk, TrackOk)
			})
		}
	})
	t.Run("Given a Try that fails at step 2", func(t *testing.T) {
		tr := RunDo(DoInput{Mode: "do", FailStep2: true, Bug: true})
		t.Run("Then Do returns the Err and the bug does not run", func(t *testing.T) {
			if tr.Outcome != "err" || !strings.Contains(tr.Error, "price list") {
				t.Fatalf("trace = %+v", tr)
			}
			wantTracks(t, tr.Steps, TrackOk, TrackErr, TrackSkipped, TrackSkipped)
		})
	})
	t.Run("Given a nil map write", func(t *testing.T) {
		t.Run("When the block runs in Do", func(t *testing.T) {
			tr := RunDo(DoInput{Mode: "do", Bug: true})
			t.Run("Then the panic escapes Do", func(t *testing.T) {
				if tr.Outcome != "crash" || !strings.Contains(tr.Error, "nil map") {
					t.Fatalf("trace = %+v", tr)
				}
				wantTracks(t, tr.Steps, TrackOk, TrackOk, TrackPanic, TrackSkipped)
			})
		})
		t.Run("When the block runs in DoRecover", func(t *testing.T) {
			tr := RunDo(DoInput{Mode: "dorecover", Bug: true})
			t.Run("Then the result is an Err with a PanicError and a stack", func(t *testing.T) {
				if tr.Outcome != "panic_error" || tr.Stack == "" {
					t.Fatalf("trace = %+v", tr)
				}
			})
		})
	})
}

func kinds(events []Event, task string) []string {
	var out []string
	for _, e := range events {
		if e.Task == task {
			out = append(out, e.Kind)
		}
	}
	return out
}

func TestTasks(t *testing.T) {
	ctx := context.Background()
	specs := func(failMiddle bool) []TaskSpec {
		return []TaskSpec{
			{Name: "a", DurationMs: 20},
			{Name: "b", DurationMs: 60, Fails: failMiddle},
			{Name: "c", DurationMs: 400},
		}
	}
	t.Run("Given three tasks that succeed", func(t *testing.T) {
		t.Run("When All runs them", func(t *testing.T) {
			tr := RunTasks(ctx, TasksInput{Mode: "all", Tasks: specs(false)})
			t.Run("Then it returns every value", func(t *testing.T) {
				if !tr.Ok || !strings.Contains(tr.Value, "c data") {
					t.Fatalf("trace = %+v", tr)
				}
			})
		})
		t.Run("When Race runs them", func(t *testing.T) {
			tr := RunTasks(ctx, TasksInput{Mode: "race", Tasks: specs(false)})
			t.Run("Then the fastest wins and the others are canceled", func(t *testing.T) {
				if !tr.Ok || tr.Value != "a data" {
					t.Fatalf("trace = %+v", tr)
				}
				if got := kinds(tr.Events, "c"); !slices.Equal(got, []string{EventStart, EventCanceled}) {
					t.Fatalf("c events = %v", got)
				}
				if tr.TotalMs >= 400 {
					t.Fatalf("total = %d ms; the slow task was not canceled", tr.TotalMs)
				}
			})
		})
	})
	t.Run("Given a middle task that fails", func(t *testing.T) {
		t.Run("When All runs them", func(t *testing.T) {
			tr := RunTasks(ctx, TasksInput{Mode: "all", Tasks: specs(true)})
			t.Run("Then it fails and cancels the slow task", func(t *testing.T) {
				if tr.Ok || !strings.Contains(tr.Error, "b: upstream") {
					t.Fatalf("trace = %+v", tr)
				}
				if got := kinds(tr.Events, "c"); !slices.Equal(got, []string{EventStart, EventCanceled}) {
					t.Fatalf("c events = %v", got)
				}
			})
		})
		t.Run("When Any runs them", func(t *testing.T) {
			tr := RunTasks(ctx, TasksInput{Mode: "any", Tasks: specs(true)})
			t.Run("Then the first success wins", func(t *testing.T) {
				if !tr.Ok || tr.Value != "a data" {
					t.Fatalf("trace = %+v", tr)
				}
			})
		})
		t.Run("When AllSettled runs them", func(t *testing.T) {
			tr := RunTasks(ctx, TasksInput{Mode: "allSettled", Tasks: specs(true)})
			t.Run("Then it reports every result and cancels nothing", func(t *testing.T) {
				if len(tr.Settled) != 3 || tr.Settled[1].Ok || !tr.Settled[2].Ok {
					t.Fatalf("settled = %+v", tr.Settled)
				}
			})
		})
	})
	t.Run("Given a context with an event hook", func(t *testing.T) {
		var got []Event
		hctx := WithEventHook(ctx, func(e Event) { got = append(got, e) })
		tr := RunTasks(hctx, TasksInput{Mode: "all", Tasks: specs(false)})
		t.Run("Then the hook sees every event in order", func(t *testing.T) {
			if len(got) != len(tr.Events) || len(got) != 6 {
				t.Fatalf("hook got %d events, trace has %d", len(got), len(tr.Events))
			}
		})
	})
	t.Run("Given a trace", func(t *testing.T) {
		tr := RunTasks(ctx, TasksInput{Mode: "all", Tasks: []TaskSpec{{DurationMs: 1}}})
		t.Run("Then it is JSON and the inputs are clamped", func(t *testing.T) {
			if _, err := json.Marshal(tr); err != nil {
				t.Fatal(err)
			}
			if tr.Tasks[0].Name != "usage" || tr.Tasks[0].DurationMs != minTaskMs {
				t.Fatalf("tasks = %+v", tr.Tasks)
			}
		})
	})
}

func TestRetry(t *testing.T) {
	ctx := context.Background()
	t.Run("Given a charge that fails twice", func(t *testing.T) {
		t.Run("When three attempts are allowed", func(t *testing.T) {
			tr := RunRetry(ctx, RetryInput{FailuresBeforeSuccess: 2, MaxAttempts: 3, BaseMs: 10})
			t.Run("Then the third attempt succeeds after two waits", func(t *testing.T) {
				if !tr.Ok || len(tr.Attempts) != 3 || !tr.Attempts[2].Ok {
					t.Fatalf("trace = %+v", tr)
				}
				if !slices.Equal(tr.PlanMs, []int64{10, 20}) {
					t.Fatalf("plan = %v", tr.PlanMs)
				}
				wait := tr.Attempts[2].StartMs - tr.Attempts[1].EndMs
				if wait < 15 {
					t.Fatalf("second wait = %d ms, want about 20", wait)
				}
			})
		})
		t.Run("When two attempts are allowed", func(t *testing.T) {
			tr := RunRetry(ctx, RetryInput{FailuresBeforeSuccess: 2, MaxAttempts: 2, BaseMs: 10})
			t.Run("Then Retry gives up", func(t *testing.T) {
				if tr.Ok || len(tr.Attempts) != 2 || !strings.Contains(tr.Error, "gave up after 2") {
					t.Fatalf("trace = %+v", tr)
				}
			})
		})
	})
	t.Run("Given a context that ends during a wait", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()
		tr := RunRetry(ctx, RetryInput{FailuresBeforeSuccess: 5, MaxAttempts: 6, BaseMs: 1000})
		t.Run("Then Retry stops early", func(t *testing.T) {
			if tr.Ok || tr.TotalMs > 1000 {
				t.Fatalf("trace = %+v", tr)
			}
		})
	})
}
