package validator

import "testing"

func TestCheckKeepsFirstErrorPerKey(t *testing.T) {
	v := New()
	v.Check(false, "email", "must be provided")
	v.Check(false, "email", "must be a valid email address")
	v.Check(true, "name", "must be provided")

	if v.Valid() {
		t.Fatal("expected validator to be invalid")
	}
	if got := v.Errors["email"]; got != "must be provided" {
		t.Errorf("email error = %q, want the first message", got)
	}
	if _, ok := v.Errors["name"]; ok {
		t.Error("passing check must not add an error")
	}
}

func TestEmailRX(t *testing.T) {
	tests := map[string]bool{
		"alice@example.com":       true,
		"a.b+tag@sub.example.org": true,
		"no-at-sign.example.com":  false,
		"two@@example.com":        false,
		"trailing@example-.com":   false,
		"":                        false,
	}
	for email, want := range tests {
		if got := Matches(email, EmailRX); got != want {
			t.Errorf("Matches(%q) = %v, want %v", email, got, want)
		}
	}
}

func TestPermittedValueAndUnique(t *testing.T) {
	if !PermittedValue("year", "id", "year", "-year") {
		t.Error("year should be permitted")
	}
	if PermittedValue("password", "id", "year") {
		t.Error("password should not be permitted")
	}
	if !Unique([]string{"drama", "comedy"}) {
		t.Error("distinct values should be unique")
	}
	if Unique([]string{"drama", "drama"}) {
		t.Error("repeated values should not be unique")
	}
}
