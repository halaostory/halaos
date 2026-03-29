# Contributing to HalaOS

Thank you for your interest in contributing to HalaOS! This guide will help you get started.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/halaos.git`
3. Create a feature branch: `git checkout -b feature/your-feature`
4. Make your changes
5. Run tests: `go test ./...`
6. Commit your changes: `git commit -m "feat: your feature description"`
7. Push to your fork: `git push origin feature/your-feature`
8. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.25+
- Node.js 22+
- Docker & Docker Compose
- PostgreSQL 16 (via Docker)
- Redis 7 (via Docker)

### Running Locally

```bash
# Start infrastructure
docker compose up -d

# Copy and configure environment
cp .env.example .env
# Edit .env with your settings (JWT_SECRET and POSTGRES_PASSWORD are required)

# Run database migrations
go run ./cmd/migrate -cmd up

# Start the API server
go run ./cmd/api

# Start the worker (in another terminal)
go run ./cmd/worker

# Start the frontend (in another terminal)
cd frontend && npm install && npm run dev
```

## Code Style

- Go: Follow standard Go conventions (`gofmt`, `go vet`)
- Frontend: Vue3 + TypeScript + NaiveUI
- Commit messages: Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, etc.)

## Pull Request Guidelines

- Keep PRs focused — one feature or fix per PR
- Include tests for new functionality
- Update documentation if needed
- Ensure all tests pass before submitting
- Write a clear PR description explaining the "why"

## Reporting Issues

- Use GitHub Issues for bug reports and feature requests
- Include steps to reproduce for bugs
- Include your environment details (OS, Go version, etc.)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
