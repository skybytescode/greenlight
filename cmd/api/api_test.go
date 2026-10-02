package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestHealthcheck(t *testing.T) {
	e := newTestEnv(t, nil)

	r := e.do(t, http.MethodGet, "/v1/healthcheck", "", nil)
	expectStatus(t, r, http.StatusOK)
	if r.body["status"] != "available" {
		t.Errorf("status = %v, want available", r.body["status"])
	}
	if env := r.body["system_info"].(map[string]any)["environment"]; env != "testing" {
		t.Errorf("environment = %v, want testing", env)
	}
}

func TestSignUpActivateAndLogIn(t *testing.T) {
	e := newTestEnv(t, nil)

	r := e.do(t, http.MethodPost, "/v1/users", "", map[string]string{
		"name": "Alice", "email": "alice@example.com", "password": "pa55word123",
	})
	expectStatus(t, r, http.StatusAccepted)
	if r.body["user"].(map[string]any)["activated"] != false {
		t.Fatal("a new user must not be activated")
	}

	e.app.wg.Wait()
	mail := e.mailer.last(t)
	if mail.recipient != "alice@example.com" || mail.template != "user_welcome.tmpl" {
		t.Fatalf("unexpected email: %+v", mail)
	}

	// Logging in works before activation, but the account cannot read movies yet.
	r = e.do(t, http.MethodPost, "/v1/tokens/authentication", "", map[string]string{
		"email": "alice@example.com", "password": "pa55word123",
	})
	expectStatus(t, r, http.StatusCreated)
	token := r.body["authentication_token"].(map[string]any)["token"].(string)
	expectStatus(t, e.do(t, http.MethodGet, "/v1/movies", token, nil), http.StatusForbidden)

	r = e.do(t, http.MethodPut, "/v1/users/activated", "", map[string]any{"token": mail.data["activationToken"]})
	expectStatus(t, r, http.StatusOK)
	expectStatus(t, e.do(t, http.MethodGet, "/v1/movies", token, nil), http.StatusOK)

	// The activation token is single use.
	r = e.do(t, http.MethodPut, "/v1/users/activated", "", map[string]any{"token": mail.data["activationToken"]})
	expectStatus(t, r, http.StatusUnprocessableEntity)
}

func TestSignUpValidation(t *testing.T) {
	e := newTestEnv(t, nil)

	r := e.do(t, http.MethodPost, "/v1/users", "", map[string]string{
		"name": "", "email": "not-an-email", "password": "short",
	})
	expectStatus(t, r, http.StatusUnprocessableEntity)
	errs := r.body["error"].(map[string]any)
	for _, field := range []string{"name", "email", "password"} {
		if _, ok := errs[field]; !ok {
			t.Errorf("expected a validation error for %s, got %v", field, errs)
		}
	}

	e.registerActivatedUser(t, "bob@example.com")
	r = e.do(t, http.MethodPost, "/v1/users", "", map[string]string{
		"name": "Bob again", "email": "BOB@example.com", "password": "pa55word123",
	})
	expectStatus(t, r, http.StatusUnprocessableEntity) // emails are case-insensitive (citext)
}

func TestAuthentication(t *testing.T) {
	e := newTestEnv(t, nil)
	e.registerActivatedUser(t, "carol@example.com")

	r := e.do(t, http.MethodPost, "/v1/tokens/authentication", "", map[string]string{
		"email": "carol@example.com", "password": "wrong-password",
	})
	expectStatus(t, r, http.StatusUnauthorized)

	expectStatus(t, e.do(t, http.MethodGet, "/v1/movies", "", nil), http.StatusUnauthorized)

	r = e.do(t, http.MethodGet, "/v1/movies", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", nil)
	expectStatus(t, r, http.StatusUnauthorized)
	if r.header.Get("WWW-Authenticate") != "Bearer" {
		t.Error("an invalid token must be answered with WWW-Authenticate: Bearer")
	}
}

func TestMovieCRUDAndPermissions(t *testing.T) {
	e := newTestEnv(t, nil)
	userID, token := e.registerActivatedUser(t, "dave@example.com")

	movie := map[string]any{"title": "Moana", "year": 2016, "runtime": "107 mins", "genres": []string{"animation", "adventure"}}

	// New users can read but not write.
	expectStatus(t, e.do(t, http.MethodPost, "/v1/movies", token, movie), http.StatusForbidden)
	if err := e.app.models.Permissions.AddForUser(userID, "movies:write"); err != nil {
		t.Fatal(err)
	}

	r := e.do(t, http.MethodPost, "/v1/movies", token, movie)
	expectStatus(t, r, http.StatusCreated)
	id := int64(r.body["movie"].(map[string]any)["id"].(float64))
	if loc := r.header.Get("Location"); loc != fmt.Sprintf("/v1/movies/%d", id) {
		t.Errorf("Location = %q", loc)
	}

	r = e.do(t, http.MethodGet, fmt.Sprintf("/v1/movies/%d", id), token, nil)
	expectStatus(t, r, http.StatusOK)
	if got := r.body["movie"].(map[string]any)["runtime"]; got != "107 mins" {
		t.Errorf("runtime = %v, want \"107 mins\"", got)
	}

	r = e.do(t, http.MethodPatch, fmt.Sprintf("/v1/movies/%d", id), token, map[string]any{"year": 1985})
	expectStatus(t, r, http.StatusOK)
	updated := r.body["movie"].(map[string]any)
	if updated["year"] != float64(1985) || updated["title"] != "Moana" || updated["version"] != float64(2) {
		t.Errorf("partial update gave %v", updated)
	}

	r = e.do(t, http.MethodPatch, fmt.Sprintf("/v1/movies/%d", id), token, map[string]any{"year": 3000})
	expectStatus(t, r, http.StatusUnprocessableEntity)

	expectStatus(t, e.do(t, http.MethodDelete, fmt.Sprintf("/v1/movies/%d", id), token, nil), http.StatusOK)
	expectStatus(t, e.do(t, http.MethodGet, fmt.Sprintf("/v1/movies/%d", id), token, nil), http.StatusNotFound)
	expectStatus(t, e.do(t, http.MethodDelete, fmt.Sprintf("/v1/movies/%d", id), token, nil), http.StatusNotFound)
}

func TestListMoviesFilteringSortingPaging(t *testing.T) {
	e := newTestEnv(t, nil)
	userID, token := e.registerActivatedUser(t, "erin@example.com")
	if err := e.app.models.Permissions.AddForUser(userID, "movies:write"); err != nil {
		t.Fatal(err)
	}

	for _, m := range []map[string]any{
		{"title": "The Breakfast Club", "year": 1985, "runtime": "96 mins", "genres": []string{"drama"}},
		{"title": "Black Panther", "year": 2018, "runtime": "134 mins", "genres": []string{"action", "adventure"}},
		{"title": "Deadpool", "year": 2016, "runtime": "108 mins", "genres": []string{"action", "comedy"}},
		{"title": "The Godfather", "year": 1972, "runtime": "175 mins", "genres": []string{"crime", "drama"}},
	} {
		expectStatus(t, e.do(t, http.MethodPost, "/v1/movies", token, m), http.StatusCreated)
	}

	titles := func(r response) []string {
		var out []string
		for _, m := range r.body["movies"].([]any) {
			out = append(out, m.(map[string]any)["title"].(string))
		}
		return out
	}

	r := e.do(t, http.MethodGet, "/v1/movies?genres=action&sort=-year", token, nil)
	expectStatus(t, r, http.StatusOK)
	if got := strings.Join(titles(r), ", "); got != "Black Panther, Deadpool" {
		t.Errorf("action movies newest first = %s", got)
	}

	r = e.do(t, http.MethodGet, "/v1/movies?title=the", token, nil)
	expectStatus(t, r, http.StatusOK)
	if got := len(titles(r)); got != 2 {
		t.Errorf("full-text search for \"the\" returned %d movies, want 2", got)
	}

	r = e.do(t, http.MethodGet, "/v1/movies?sort=year&page=2&page_size=3", token, nil)
	expectStatus(t, r, http.StatusOK)
	if got := strings.Join(titles(r), ", "); got != "Black Panther" {
		t.Errorf("page 2 sorted by year = %s", got)
	}
	meta := r.body["metadata"].(map[string]any)
	if meta["total_records"] != float64(4) || meta["last_page"] != float64(2) {
		t.Errorf("metadata = %v", meta)
	}

	r = e.do(t, http.MethodGet, "/v1/movies?sort=password&page_size=1000", token, nil)
	expectStatus(t, r, http.StatusUnprocessableEntity)
}

func TestBadRequestBodies(t *testing.T) {
	e := newTestEnv(t, nil)

	tests := []struct {
		name string
		body string
		want string
	}{
		{"malformed JSON", `{"name": "Alice",`, "badly-formed JSON"},
		{"wrong type", `{"name": 123}`, `incorrect JSON type for field "name"`},
		{"unknown field", `{"name": "Alice", "admin": true}`, `unknown key "admin"`},
		{"two JSON values", `{"name": "Alice"}{"name": "Bob"}`, "single JSON value"},
		{"empty body", ``, "must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := e.do(t, http.MethodPost, "/v1/users", "", tt.body)
			expectStatus(t, r, http.StatusBadRequest)
			if msg, _ := r.body["error"].(string); !strings.Contains(msg, tt.want) {
				t.Errorf("error = %q, want it to mention %q", msg, tt.want)
			}
		})
	}

	expectStatus(t, e.do(t, http.MethodGet, "/v1/nothing-here", "", nil), http.StatusNotFound)
	expectStatus(t, e.do(t, http.MethodDelete, "/v1/healthcheck", "", nil), http.StatusMethodNotAllowed)
}

func TestRateLimit(t *testing.T) {
	e := newTestEnv(t, func(cfg *config) {
		cfg.limiter.enabled = true
		cfg.limiter.rps = 1
		cfg.limiter.burst = 3
	})

	for i := 1; i <= 3; i++ {
		expectStatus(t, e.do(t, http.MethodGet, "/v1/healthcheck", "", nil), http.StatusOK)
	}
	expectStatus(t, e.do(t, http.MethodGet, "/v1/healthcheck", "", nil), http.StatusTooManyRequests)
}
