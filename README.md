# Greenlight

[![CI](https://github.com/skybytescode/greenlight/actions/workflows/ci.yml/badge.svg)](https://github.com/skybytescode/greenlight/actions/workflows/ci.yml)

A JSON API for retrieving and managing information about movies, built while working through
Alex Edwards' *Let's Go Further*. Think of the core functionality as being a bit like the
[Open Movie Database API](https://www.omdbapi.com/).

## Endpoints

| Method | URL Pattern                  | Action                                |
| ------ | ----------------------------- | -------------------------------------- |
| GET    | /v1/healthcheck               | Show application health and version    |
| GET    | /v1/movies                    | Show the details of all movies         |
| POST   | /v1/movies                    | Create a new movie                     |
| GET    | /v1/movies/:id                | Show the details of a specific movie   |
| PATCH  | /v1/movies/:id                | Update the details of a specific movie |
| DELETE | /v1/movies/:id                | Delete a specific movie                |
| POST   | /v1/users                     | Register a new user                    |
| PUT    | /v1/users/activated           | Activate a specific user               |
| PUT    | /v1/users/password            | Update the password for a user         |
| POST   | /v1/tokens/authentication     | Generate a new authentication token    |
| POST   | /v1/tokens/password-reset     | Generate a new password-reset token    |
| POST   | /v1/tokens/activation         | Generate a new activation token        |
| GET    | /debug/vars                   | Display application metrics            |

## Run everything with Docker

```bash
docker compose up --build
```

This starts PostgreSQL, applies the migrations, runs the API on `:4000` and
starts [Mailpit](https://mailpit.axllent.org/), a local mail server. Emails the
API sends, such as the activation email after sign-up, show up in its inbox at
http://localhost:8025.

```bash
curl -X POST localhost:4000/v1/users \
  -d '{"name":"Maria","email":"maria@example.com","password":"pa55word123"}'
# open http://localhost:8025, copy the token from the welcome email, then:
curl -X PUT localhost:4000/v1/users/activated -d '{"token":"<token from the email>"}'
```

## Tests

```bash
go test ./...   # unit tests; the integration tests skip themselves
```

The integration tests in `cmd/api` run the real routes and middleware against
PostgreSQL: sign-up and activation (with the activation email captured by a
fake mailer), authentication, permissions, movie CRUD with optimistic locking,
filtering, sorting and paging, malformed request bodies and rate limiting. They
need an empty database they are allowed to wipe:

```bash
docker compose up -d db
PGPASSWORD=pa55word psql -h localhost -U greenlight -c 'CREATE DATABASE greenlight_test'
GREENLIGHT_TEST_DB_DSN='postgres://greenlight:pa55word@localhost/greenlight_test?sslmode=disable' \
  go test -race ./...
```

[GitHub Actions](.github/workflows/ci.yml) runs gofmt, `go vet`, staticcheck
and all the tests against a PostgreSQL service container on every push and pull
request, then builds the Docker image.

## Running locally without Docker for the API

Requires Go 1.26+ and PostgreSQL (`docker compose up -d db` starts one).

```bash
cp .envrc.example .envrc
# edit .envrc with real values, then load it into your shell, e.g.
source .envrc

docker compose up -d db       # starts Postgres on localhost:5432

# install golang-migrate: https://github.com/golang-migrate/migrate
make db/migrations/up

make run/api
```

The API listens on `:4000` by default.

```bash
curl localhost:4000/v1/healthcheck
```

## Project layout

- `cmd/api` — application-specific code: routing, handlers, middleware, config, server lifecycle.
- `internal/data` — database models (movies, users, tokens, permissions) and validation.
- `internal/validator` — small reusable validation helper.
- `internal/mailer` — transactional email sending with embedded templates.
- `internal/jsonlog` — structured (JSON) leveled logging.
- `internal/vcs` — build version info derived from VCS metadata.
- `migrations` — SQL schema migrations (golang-migrate format).
- `remote` — reference-only production deployment assets (systemd unit, Caddyfile, provisioning
  script) — not executed as part of this build; adapt and run manually against a real server if
  you deploy this.

## Notes on scope

This implements the core of the book's build end-to-end: JSON API conventions, PostgreSQL-backed
CRUD with filtering/sorting/pagination and optimistic concurrency, structured logging and panic
recovery, per-client rate limiting, user registration/activation, token-based authentication,
permission-based authorization, CORS, and `/debug/vars` metrics. The final deployment chapter
(provisioning a real Digital Ocean droplet, live TLS via a real domain, running as a systemd
service on that host) is represented as code/config under `remote/` rather than actually executed,
since it requires a real external server.
