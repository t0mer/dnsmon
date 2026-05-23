# Contributing to dnsmon

Thank you for your interest in contributing to dnsmon! This document covers how to set up the development environment, our branching strategy, commit conventions, and the pull request checklist.

---

## Development Setup

### Prerequisites

- **Go 1.23+** — [install guide](https://go.dev/doc/install)
- **Node.js 20+** — for building Tailwind CSS (build-time only; not required at runtime)
- **golangci-lint** — installed automatically by `make lint`
- **Docker** — optional, for running integration tests with Postgres/Redis

### Clone and build

```bash
git clone https://github.com/t0mer/dnsmon.git
cd dnsmon

# Install frontend build deps
cd web && npm ci && cd ..

# Build frontend (Tailwind CSS)
make ui

# Build binary
make build

# Run tests
make test

# Run with hot-reload (requires `air`)
make dev
```

### Running the server locally

```bash
./bin/dnsmon --config config/config.example.yaml
# or with env vars:
DNSMON_STORAGE_DRIVER=none DNSMON_CACHE_DRIVER=memory ./bin/dnsmon
```

Open http://localhost:8080.

---

## Code Style

- Format with `gofmt -s` or `gofumpt` (the linter enforces this).
- Run `make lint` before submitting. All lint warnings must be resolved.
- Follow the conventions in `CLAUDE.md` §10.
- Errors: always wrap with `fmt.Errorf("context: %w", err)`.
- Contexts: every exported function that does I/O takes `context.Context` as first arg.
- Logging: use `slog` only; never `fmt.Println` or `log.Printf` in non-main code.
- No global mutable state except the logger and embedded assets.

---

## Branch Naming

We use trunk-based development. Create short-lived feature branches off `main`:

| Type | Pattern | Example |
|---|---|---|
| Feature | `feat/<short-description>` | `feat/export-pdf` |
| Bug fix | `fix/<short-description>` | `fix/timeout-race-condition` |
| Chore | `chore/<short-description>` | `chore/update-deps` |
| Documentation | `docs/<short-description>` | `docs/deployment-guide` |
| Refactor | `refactor/<short-description>` | `refactor/checker-fan-out` |

Keep branches small and focused. Prefer multiple small PRs over one large PR.

---

## Commit Message Format

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short summary>

[optional body]

[optional footer]
```

### Types

| Type | When to use |
|---|---|
| `feat` | A new feature |
| `fix` | A bug fix |
| `chore` | Dependency updates, build changes, tooling |
| `docs` | Documentation only |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `test` | Adding or updating tests |
| `perf` | Performance improvement |
| `ci` | CI/CD pipeline changes |

### Scope (optional)

Use the package name: `dnsclient`, `checker`, `api`, `cache`, `storage`, `web`, `resolvers`, etc.

### Examples

```
feat(api): add WebSocket streaming endpoint for live checks
fix(dnsclient): handle SERVFAIL responses without panicking
chore: update miekg/dns to v1.1.62
docs(deployment): add Kubernetes section to DEPLOYMENT.md
test(checker): add table-driven tests for consensus calculation
refactor(cache): extract Redis implementation to separate file
```

---

## Pull Request Checklist

Before opening a PR, verify all of the following:

- [ ] `make lint` passes with zero warnings
- [ ] `make test` passes (all unit tests green)
- [ ] New code has at least one unit test (table-driven where applicable)
- [ ] New API endpoints have an integration test in `internal/api/v1/handlers_test.go`
- [ ] New features include a doc comment on every exported type and function
- [ ] If a new HTTP endpoint was added: `internal/api/docs/openapi.yaml` is updated
- [ ] If a new config option was added: `config/config.example.yaml`, `internal/config/defaults.go`, and `docs/CONFIGURATION.md` are all updated
- [ ] No new external dependencies without justification in `CLAUDE.md` §2
- [ ] No `fmt.Println`, `log.Printf`, or `panic` outside `main`
- [ ] No hard-coded secrets or credentials
- [ ] The PR title follows Conventional Commits format
- [ ] The PR description explains *why* the change is needed, not just what it does

---

## Tests

### Unit tests

Run:
```bash
make test
# or
go test -race -count=1 ./...
```

Test files live next to the code they test (`foo.go` → `foo_test.go`). Use table-driven tests where multiple inputs/outputs are being validated. Mock DNS queries using a local `*dns.Server` from `miekg/dns` — do not make real DNS queries in tests.

### Integration tests

Run:
```bash
make test-integration
```

This starts the binary with an in-memory SQLite database, exercises the HTTP API, and shuts down. See `scripts/test-integration.sh`.

---

## Adding a New Resolver

Edit `internal/resolvers/resolvers.json` directly, or run the update script:

```bash
./scripts/update-resolvers.sh --dry-run   # preview changes
./scripts/update-resolvers.sh             # update in place
```

Each resolver must have: `id`, `name`, `ip`, `port`, `protocol`, `country`, `lat`, `lng`.

---

## Dependency Policy

Before adding a new Go dependency:
1. Check if the standard library or an existing dependency can solve the problem.
2. Evaluate the dependency's maintenance status, license, and transitive deps.
3. Add an entry to `CLAUDE.md` §2 with a justification.
4. Run `go mod tidy` after adding.

---

## Reporting Issues

Open a GitHub issue with:
- dnsmon version (`./bin/dnsmon --version`)
- Steps to reproduce
- Expected vs. actual behavior
- Relevant log output (sanitize any sensitive data)

---

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
