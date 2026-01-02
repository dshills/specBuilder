# Contributing to Spec Builder

Thank you for your interest in contributing to Spec Builder! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## How to Contribute

### Reporting Bugs

1. Check the [issue tracker](https://github.com/dshills/specbuilder/issues) to see if the bug has already been reported
2. If not, create a new issue with:
   - A clear, descriptive title
   - Steps to reproduce the behavior
   - Expected behavior vs actual behavior
   - Your environment (OS, Go version, Node version)
   - Any relevant logs or error messages

### Suggesting Features

1. Check existing issues to avoid duplicates
2. Create a new issue with:
   - A clear description of the feature
   - The problem it solves or use case it enables
   - Any implementation ideas (optional)

### Pull Requests

1. Fork the repository
2. Create a feature branch from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. Make your changes following the coding standards below
4. Write or update tests as needed
5. Run the test suite:
   ```bash
   make test
   ```
6. Commit your changes with a clear message
7. Push to your fork and submit a pull request

## Development Setup

### Prerequisites

- Go 1.22+
- Node.js 18+
- An LLM API key (Gemini, OpenAI, or Anthropic)

### Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/specbuilder.git
cd specbuilder

# Install dependencies
cd frontend && npm install && cd ..

# Set up environment
export GEMINI_API_KEY="your-key"  # or OPENAI_API_KEY

# Run tests to verify setup
make test
```

## Coding Standards

### Go (Backend)

- Follow standard Go conventions and idioms
- Run `make fmt` before committing
- Run `make lint` if you have golangci-lint installed
- Write table-driven tests where appropriate
- Keep functions focused and reasonably sized

### TypeScript/React (Frontend)

- Follow the existing code style
- Run `npm run lint` before committing
- Use TypeScript types appropriately
- Prefer functional components with hooks

### Domain Invariants

These are critical rules that must be preserved:

1. **Answer immutability** - Editing an answer creates a new version with `supersedes` link. Never mutate existing answers.

2. **Snapshot append-only** - Snapshots are never updated in place. Each compilation creates a new snapshot.

3. **Trace coverage** - Every populated spec field must have provenance in the trace.

4. **Deterministic compilation** - Same inputs must produce identical spec output.

5. **Backend-only LLM calls** - The frontend never calls LLMs directly.

See [CLAUDE.md](CLAUDE.md) for detailed architecture and invariant documentation.

## Testing

### Backend

```bash
# Run all backend tests
make backend-test

# Run a specific test
cd backend && go test -run TestName ./path/to/package
```

### Frontend

```bash
# Run all frontend tests
make frontend-test

# Run in watch mode
cd frontend && npm run test:watch
```

## Commit Messages

- Use clear, descriptive commit messages
- Start with a verb in present tense (e.g., "Add", "Fix", "Update")
- Keep the first line under 72 characters
- Reference issues when applicable (e.g., "Fix #123")

Examples:
```
Add snapshot diff endpoint
Fix answer versioning when superseding
Update README with contribution guidelines
```

## Questions?

If you have questions about contributing, feel free to open an issue with the "question" label.

Thank you for contributing!
