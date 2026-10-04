package monadz_test

import (
	"reflect"
	"strings"
	"testing"
)

// This file holds the shared assertion helpers of the root tests.

// wantEq fails the test if got != want.
func wantEq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// wantDeep fails the test if got and want are not deeply equal.
func wantDeep(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// wantTrue fails the test if cond is false.
func wantTrue(t *testing.T, cond bool, what string) {
	t.Helper()
	if !cond {
		t.Errorf("want %s to be true, got false", what)
	}
}

// isNil reports whether v is nil, or a typed nil pointer, slice, map,
// func, chan, or interface.
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan, reflect.Interface:
		return rv.IsNil()
	}
	return false
}

// wantNil fails the test if v is not nil.
func wantNil(t *testing.T, v any) {
	t.Helper()
	if !isNil(v) {
		t.Errorf("got %#v, want nil", v)
	}
}

// wantNotNil fails the test if v is nil.
func wantNotNil(t *testing.T, v any) {
	t.Helper()
	if isNil(v) {
		t.Errorf("got nil, want a non-nil value")
	}
}

// wantNotEmpty fails the test if b is empty.
func wantNotEmpty(t *testing.T, b []byte) {
	t.Helper()
	if len(b) == 0 {
		t.Errorf("got an empty slice, want a non-empty slice")
	}
}

// wantContains fails the test if s does not contain sub.
func wantContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("got %q, want it to contain %q", s, sub)
	}
}

// wantNotContains fails the test if s contains sub.
func wantNotContains(t *testing.T, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Errorf("got %q, want it not to contain %q", s, sub)
	}
}

// wantPrefix fails the test if s does not start with prefix.
func wantPrefix(t *testing.T, s, prefix string) {
	t.Helper()
	if !strings.HasPrefix(s, prefix) {
		t.Errorf("got %q, want the prefix %q", s, prefix)
	}
}

// recovered runs f and returns the value that f panics with, or nil.
func recovered(f func()) (p any) {
	defer func() { p = recover() }()
	f()
	return nil
}

// mustPanic fails the test if f does not panic, and returns the value that
// f panics with.
func mustPanic(t *testing.T, f func()) (p any) {
	t.Helper()
	panicked := true
	func() {
		defer func() { p = recover() }()
		f()
		panicked = false
	}()
	if !panicked {
		t.Errorf("got no panic, want a panic")
	}
	return p
}

// wantPanic fails the test if f does not panic with a value deeply equal
// to want.
func wantPanic(t *testing.T, f func(), want any) {
	t.Helper()
	if got := mustPanic(t, f); !reflect.DeepEqual(got, want) {
		t.Errorf("got panic %#v, want panic %#v", got, want)
	}
}

// wantNoPanic fails the test if f panics.
func wantNoPanic(t *testing.T, f func()) {
	t.Helper()
	if p := recovered(f); p != nil {
		t.Errorf("want no panic, got panic %v", p)
	}
}
