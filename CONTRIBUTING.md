# Contributing to codebak

Thank you for your interest in contributing to codebak!

## Development Setup

```bash
# Clone the repository
git clone https://github.com/jmcdonald/codebak.git
cd codebak

# Build
make build

# Run tests
make test

# Install locally
make install
```

## Testing Policy

This section defines what code MUST, SHOULD, and SHOULD NOT be tested.

### Testing Rules

| Category | Rule | Reason |
|----------|------|--------|
| `internal/backup/` | **MUST TEST** | Core business logic for backup operations |
| `internal/config/` | **MUST TEST** | Configuration loading and validation |
| `internal/manifest/` | **MUST TEST** | Manifest generation and checksum verification |
| `internal/recovery/` | **MUST TEST** | Recovery operations - critical for data integrity |
| `internal/cli/` | **MUST TEST** | Command handlers - test via mocked dependencies |
| `internal/tui/` | **MUST TEST** | Pure functions and state logic |
| `internal/mocks/` | **MUST TEST** | Verify mock behavior matches contracts |
| `internal/adapters/` | **DO NOT TEST** | Thin wrappers around stdlib/OS - tested via integration |
| `internal/launchd/` | **DO NOT TEST** | macOS-specific system calls requiring permissions |
| `internal/ports/` | **DO NOT TEST** | Interface definitions only |

### Coverage Target

**All testable code paths must have 95%+ coverage.** OS error fallbacks and untestable paths are excluded.

| Package | Target | Notes |
|---------|--------|-------|
| `backup` | 95%+ | Excluding manifest/checksum OS errors |
| `config` | 85%+ | Excluding os.UserHomeDir fallbacks |
| `manifest` | 95%+ | ✓ Achieved |
| `recovery` | 95%+ | ✓ Achieved |
| `cli` | 95%+ | ✓ Achieved |
| `tui` | 95%+ | ✓ Achieved |
| `mocks` | 100% | ✓ Achieved |

### What NOT to Test

1. **Adapter implementations** (`internal/adapters/*`)
   - Thin wrappers around `os`, `archive/zip`, `os/exec`
   - Testing duplicates stdlib testing
   - Covered by integration tests

2. **Launchd operations** (`internal/launchd/`)
   - Requires macOS-specific permissions
   - Requires launchctl execution
   - Tested manually

3. **Render functions** (any `render*` or `View()` methods in TUI)
   - Output is visual formatting
   - Tests are brittle (break on style changes)
   - Low value relative to effort

4. **Interface definitions** (`internal/ports/`)
   - No logic to test
   - Contracts verified by implementations

5. **OS error fallbacks** (`os.UserHomeDir()` errors, etc.)
   - These are system-level error handlers for rare OS failures
   - Cannot be triggered in unit tests without mocking stdlib
   - Extremely rare in practice (e.g., no HOME environment variable)
   - Covered by defensive programming, not automated tests

### Test Patterns

Use table-driven tests for functions with multiple input scenarios:

```go
func TestTruncate(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        maxLen   int
        expected string
    }{
        {"short string", "hello", 10, "hello"},
        {"exact length", "hello", 5, "hello"},
        {"truncated", "hello world", 8, "hello..."},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := truncate(tt.input, tt.maxLen)
            if got != tt.expected {
                t.Errorf("truncate(%q, %d) = %q, want %q",
                    tt.input, tt.maxLen, got, tt.expected)
            }
        })
    }
}
```

## Making Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run tests: `make test`
5. Run linter: `make lint`
6. Commit with a descriptive message
7. Push to your fork
8. Open a Pull Request

## Code Style

- Follow standard Go conventions
- Run `make lint` before submitting (requires golangci-lint)
- Add tests for new functionality (see Testing Policy above)
- Update documentation as needed

## Pull Request Guidelines

- Keep PRs focused on a single change
- Include tests for new features
- Update README.md if adding new commands
- Reference any related issues

## Reporting Issues

- Use the issue templates when available
- Include your OS, Go version, and codebak version
- Provide steps to reproduce the issue
- Include relevant error messages or logs

## Questions?

Open an issue with the "question" label.
