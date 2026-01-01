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

## Making Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run tests: `make test`
5. Commit with a descriptive message
6. Push to your fork
7. Open a Pull Request

## Code Style

- Follow standard Go conventions
- Run `make lint` before submitting (requires golangci-lint)
- Add tests for new functionality
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
