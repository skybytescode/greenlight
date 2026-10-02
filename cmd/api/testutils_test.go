package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/skybytescode/greenlight/internal/data"
	"github.com/skybytescode/greenlight/internal/jsonlog"
)

// The integration tests run the real handlers against PostgreSQL. They need
// GREENLIGHT_TEST_DB_DSN to point at a database they may wipe; without it
// they are skipped, so `go test ./...` still works with no database.
const testDSNEnv = "GREENLIGHT_TEST_DB_DSN"

type sentMail struct {
	recipient string
	template  string
	data      map[string]any
}

// fakeMailer records emails instead of sending them.
type fakeMailer struct {
	mu   sync.Mutex
	sent []sentMail
}

func (m *fakeMailer) Send(recipient, templateFile string, data any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{recipient, templateFile, data.(map[string]any)})
	return nil
}

func (m *fakeMailer) last(t *testing.T) sentMail {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		t.Fatal("no email was sent")
	}
	return m.sent[len(m.sent)-1]
}

type testEnv struct {
	app    *application
	mailer *fakeMailer
	server *httptest.Server
}

// newTestEnv gives each test an empty, fully migrated database and a server
// running the application's real routes and middleware.
func newTestEnv(t *testing.T, configure func(*config)) *testEnv {
	t.Helper()

	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s is not set; skipping integration test", testDSNEnv)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	resetDatabase(t, db)

	var cfg config
	cfg.env = "testing"
	cfg.limiter.enabled = false
	if configure != nil {
		configure(&cfg)
	}

	mailer := &fakeMailer{}
	app := &application{
		config: cfg,
		logger: jsonlog.New(io.Discard, jsonlog.LevelInfo),
		models: data.NewModels(db),
		mailer: mailer,
	}

	server := httptest.NewServer(app.routes())
	t.Cleanup(server.Close)

	return &testEnv{app: app, mailer: mailer, server: server}
}

// resetDatabase drops everything and applies the up migrations in order.
func resetDatabase(t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}

	files, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	sort.Strings(files)
	for _, file := range files {
		script, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(script)); err != nil {
			t.Fatalf("%s: %v", filepath.Base(file), err)
		}
	}
}

type response struct {
	status int
	header http.Header
	body   map[string]any
}

// do sends a JSON request; token, when set, is sent as a Bearer token.
func (e *testEnv) do(t *testing.T, method, path, token string, body any) response {
	t.Helper()

	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = bytes.NewBufferString(b)
	default:
		payload, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := e.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	var decoded map[string]any
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil && err != io.EOF {
		t.Fatalf("%s %s: response is not JSON: %v", method, path, err)
	}
	return response{status: res.StatusCode, header: res.Header, body: decoded}
}

func expectStatus(t *testing.T, r response, want int) {
	t.Helper()
	if r.status != want {
		t.Fatalf("status = %d, want %d; body: %v", r.status, want, r.body)
	}
}

// registerActivatedUser goes through the public sign-up flow and returns an
// authentication token for the new, activated user.
func (e *testEnv) registerActivatedUser(t *testing.T, email string) (int64, string) {
	t.Helper()

	r := e.do(t, http.MethodPost, "/v1/users", "", map[string]string{
		"name": "Test User", "email": email, "password": "pa55word123",
	})
	expectStatus(t, r, http.StatusAccepted)
	userID := int64(r.body["user"].(map[string]any)["id"].(float64))

	e.app.wg.Wait() // the welcome email is sent in the background
	activation := e.mailer.last(t).data["activationToken"].(string)

	r = e.do(t, http.MethodPut, "/v1/users/activated", "", map[string]string{"token": activation})
	expectStatus(t, r, http.StatusOK)

	r = e.do(t, http.MethodPost, "/v1/tokens/authentication", "", map[string]string{
		"email": email, "password": "pa55word123",
	})
	expectStatus(t, r, http.StatusCreated)
	return userID, r.body["authentication_token"].(map[string]any)["token"].(string)
}
