package data

import (
	"strings"
	"testing"
	"time"

	"github.com/skybytescode/greenlight/internal/validator"
)

func TestValidateMovie(t *testing.T) {
	valid := func() *Movie {
		return &Movie{Title: "Moana", Year: 2016, Runtime: 107, Genres: []string{"animation", "adventure"}}
	}
	tests := []struct {
		name    string
		change  func(*Movie)
		wantKey string
	}{
		{"valid", func(*Movie) {}, ""},
		{"missing title", func(m *Movie) { m.Title = "" }, "title"},
		{"title too long", func(m *Movie) { m.Title = strings.Repeat("a", 501) }, "title"},
		{"before cinema", func(m *Movie) { m.Year = 1887 }, "year"},
		{"in the future", func(m *Movie) { m.Year = int32(time.Now().Year() + 1) }, "year"},
		{"negative runtime", func(m *Movie) { m.Runtime = -1 }, "runtime"},
		{"no genres", func(m *Movie) { m.Genres = []string{} }, "genres"},
		{"too many genres", func(m *Movie) { m.Genres = []string{"a", "b", "c", "d", "e", "f"} }, "genres"},
		{"duplicate genres", func(m *Movie) { m.Genres = []string{"drama", "drama"} }, "genres"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := valid()
			tt.change(m)
			v := validator.New()
			ValidateMovie(v, m)
			if tt.wantKey == "" && !v.Valid() {
				t.Fatalf("unexpected errors: %v", v.Errors)
			}
			if _, ok := v.Errors[tt.wantKey]; tt.wantKey != "" && !ok {
				t.Fatalf("expected an error for %q, got %v", tt.wantKey, v.Errors)
			}
		})
	}
}

func TestPasswordSetAndMatches(t *testing.T) {
	var p password
	if err := p.Set("pa55word123"); err != nil {
		t.Fatal(err)
	}
	if ok, err := p.Matches("pa55word123"); err != nil || !ok {
		t.Fatalf("correct password: ok=%v err=%v", ok, err)
	}
	if ok, err := p.Matches("wrong-password"); err != nil || ok {
		t.Fatalf("wrong password: ok=%v err=%v", ok, err)
	}
}

func TestValidateUser(t *testing.T) {
	user := &User{Name: "Alice", Email: "not-an-email"}
	if err := user.Password.Set("short"); err != nil {
		t.Fatal(err)
	}
	v := validator.New()
	ValidateUser(v, user)
	for _, key := range []string{"email", "password"} {
		if _, ok := v.Errors[key]; !ok {
			t.Errorf("expected an error for %q, got %v", key, v.Errors)
		}
	}
	if _, ok := v.Errors["name"]; ok {
		t.Error("name is valid and should have no error")
	}
}

func TestRuntimeJSON(t *testing.T) {
	r := Runtime(102)
	b, err := r.MarshalJSON()
	if err != nil || string(b) != `"102 mins"` {
		t.Fatalf("MarshalJSON = %s, %v", b, err)
	}
	var back Runtime
	if err := back.UnmarshalJSON([]byte(`"102 mins"`)); err != nil || back != 102 {
		t.Fatalf("UnmarshalJSON = %d, %v", back, err)
	}
	if err := back.UnmarshalJSON([]byte(`"102 minutes"`)); err == nil {
		t.Fatal("expected an error for a badly formatted runtime")
	}
}
