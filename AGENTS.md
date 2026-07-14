# Repository Guidelines

## Project Structure & Module Organization

This is a Go module named `dockertui`. The application entry point and core models live at the repository root:

- `main.go`, `model.go`: Bubble Tea program setup, state, and update/view flow.
- `docker.go`, `container.go`, `service.go`, `service_inspect.go`: Docker client integration and domain behavior.
- `internal/component/`: reusable TUI components such as panels, tables, overlays, filters, dialogs, and log/inspect views.
- `main_test.go`: current unit and rendering tests.

Keep shared UI primitives in `internal/component`; keep Docker-specific logic in root-level domain files unless a larger package split becomes necessary.

## Build, Test, and Development Commands

- `go test ./...`: run all tests in the module.
- `go test ./... -run TestName`: run a focused test while iterating.
- `go build ./...`: compile all packages.
- `go run .`: launch the Docker TUI locally.
- `go mod tidy`: update module requirements after dependency changes.

Running the app requires a reachable Docker daemon and normal Docker client permissions.

## Coding Style & Naming Conventions

Use standard Go formatting: run `gofmt` on changed `.go` files before committing. Follow idiomatic Go naming: exported identifiers use `PascalCase`; unexported identifiers use `camelCase`; tests use `TestBehaviorDescription`.

Prefer small, focused functions that match the existing Bubble Tea message/update style. Keep terminal text and component rendering deterministic where possible so view tests can assert against stable strings.

## Testing Guidelines

Tests use Go's standard `testing` package. Existing coverage focuses on model transitions, key handling, filtering, and rendered terminal output. Add tests near related behavior in `main_test.go` or create `*_test.go` files beside new packages.

When changing UI output, update or add assertions for visible labels, layout markers, and mode transitions. Run `go test ./...` before submitting changes.

## Commit & Pull Request Guidelines

Recent commits use short subjects, often Conventional Commit prefixes such as `feat:` and `refactor:`. Prefer concise, imperative subjects, for example `feat: add service log filter` or `fix: preserve table selection`.

Pull requests should include a brief description, test results, and screenshots or terminal recordings for visible TUI changes. Link related issues when available and call out Docker daemon assumptions or manual verification steps.

## Security & Configuration Tips

Do not commit local Docker credentials, daemon socket overrides, or machine-specific configuration. Treat container and service inspect output as potentially sensitive when sharing logs or screenshots.
