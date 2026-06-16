# Contributing to cost-dcm-provider

Thank you for your interest in contributing!

## Getting Started

1. Fork the repository and clone your fork.
2. Install Go 1.25+ and ensure `CGO_ENABLED=1` (required for SQLite).
3. Copy `.env.example` (if present) or set the required environment variables (see README).
4. Run `go build ./...` to verify the build.
5. Run `go test ./...` to verify tests pass.

## Development Workflow

1. Create a feature branch from `main`.
2. Make your changes with clear, atomic commits.
3. Add or update tests for any new functionality.
4. Ensure `go vet ./...` passes with no warnings.
5. Open a pull request against `main`.

## Code Style

- Follow standard Go conventions (`gofmt`, `goimports`).
- Use `slog` for structured logging (JSON output).
- Use `context.Context` as the first parameter for functions that do I/O.
- Avoid adding comments that merely restate what the code does.

## Commit Messages

Use the [DCO sign-off](https://developercertificate.org/) on all commits:

```
git commit -s -m "Short description of change"
```

## Reporting Issues

Open a GitHub issue with:
- Steps to reproduce.
- Expected vs. actual behavior.
- Go version and OS.

For security issues, see [SECURITY.md](SECURITY.md).
