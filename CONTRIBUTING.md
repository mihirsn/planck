# Contributing to Planck

Thank you for your interest in contributing! Planck is a focused, lightweight tool — and we want to keep it that way. Please read this guide before opening a PR.

---

## Philosophy

> Simplicity, readability, and minimal dependencies.

Before adding a feature, ask: *Does this belong in a single-binary CLI tool?* If the answer requires a database, a background daemon, or a web server — it probably doesn't belong in V1.

---

## Getting Started

### Prerequisites

- Go 1.21+
- `make`
- `golangci-lint` (for linting)

### Setup

```bash
git clone https://github.com/mihirsn/planck.git
cd planck
go mod tidy
make build
```

### Run locally

```bash
make run ARGS="analyze sample-logs/app.log"
```

---

## Development Workflow

1. **Fork** the repository
2. **Create a branch** from `main`:
   ```bash
   git checkout -b feat/your-feature-name
   ```
3. **Make your changes** — keep them focused and minimal
4. **Write tests** — we enforce ≥ 90% test coverage
5. **Run the full check suite**:
   ```bash
   make test
   make coverage-check
   make lint
   ```
6. **Open a PR** using the PR template

---

## Code Standards

- **Formatting**: Code must pass `gofmt`. Run `gofmt -w .` before committing.
- **Imports**: Use `goimports` for import ordering.
- **Linting**: All `golangci-lint` checks must pass.
- **Tests**: New code must include tests. Coverage must stay ≥ 90%.
- **Comments**: All exported functions and types must have godoc comments.
- **Commits**: Use [Conventional Commits](https://www.conventionalcommits.org/) format:
  - `feat:` — new feature
  - `fix:` — bug fix
  - `docs:` — documentation only
  - `test:` — tests only
  - `refactor:` — no behavior change
  - `ci:` — CI/build changes

---

## What We Accept

✅ Bug fixes  
✅ Performance improvements to existing features  
✅ Documentation improvements  
✅ Test coverage improvements  
✅ New features listed on the roadmap  

---

## What We Don't Accept (in V1)

❌ Web UI / TUI / dashboards  
❌ Database or persistent storage  
❌ AI/LLM integration  
❌ Kubernetes / OpenTelemetry support  
❌ Heavy external dependencies  

If you have a big idea, please **open an issue first** to discuss it before writing code.

---

## Reporting Bugs

Use the [Bug Report template](.github/ISSUE_TEMPLATE/bug_report.md).

## Requesting Features

Use the [Feature Request template](.github/ISSUE_TEMPLATE/feature_request.md).

---

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
