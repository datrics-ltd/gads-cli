# Contributing to gads

Thanks for your interest in contributing! Here's how to get involved.

## Issues

Found a bug or have a feature request? [Open an issue](https://github.com/datrics-ltd/gads-cli/issues). Include:

- What you expected to happen
- What actually happened
- Steps to reproduce (if it's a bug)
- Your OS and `gads version` output

## Pull Requests

1. **Fork** the repo and create a branch from `main`
2. **Name your branch** descriptively: `feat/description`, `fix/description`, `docs/description`
3. **Write tests** if you're adding or changing functionality
4. **Use conventional commits**: `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`
5. **Run tests** before submitting: `go test ./...`
6. **Open a PR** against `main` with a clear description of what and why

## Development

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/gads-cli.git
cd gads-cli

# Build
go build -o gads .

# Run tests
go test ./...

# Build with version tag
go build -ldflags "-s -w -X main.version=dev" -o gads .
```

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Keep functions focused and well-named
- Add comments for anything non-obvious
- Error messages should be actionable — tell the user what to do

## Scope

This CLI wraps the Google Ads REST API. Contributions that align with that goal are welcome. If you're unsure whether something fits, open an issue first to discuss.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
