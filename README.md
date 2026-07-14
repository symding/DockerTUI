# Docker TUI

Docker TUI is a terminal interface for browsing and operating Docker containers and Swarm services. It is built with Go, Bubble Tea, Bubbles, Lip Gloss, and the Docker client SDK.

## Features

- List Docker containers with ID, name, image, command, age, status, and ports.
- Filter container and service lists from inside the TUI.
- Inspect containers, including runtime information, environment variables, and resource usage charts.
- Follow container logs.
- Start, stop, restart, and kill containers.
- List Docker Swarm services with image and replica status.
- Inspect services, tasks, environment variables, mounts, and placement constraints.
- Scale services, remove services, and edit selected service inspect fields.
- Follow logs for service tasks.

## Requirements

- Go 1.25 or newer.
- Docker CLI available on `PATH`.
- A reachable Docker daemon.
- Docker permissions for the current user.

Service features require Docker Swarm APIs and only work when the target Docker daemon has services available.

## Installation

Clone the repository and build the binary:

```sh
go build -o dockertui .
```

Run it from the repository:

```sh
./dockertui
```

You can also run without creating a binary:

```sh
go run .
```

## Usage

The app starts on the container list. Use the navigation bar to switch between containers and services.

Common keys:

| Key | Action |
| --- | --- |
| `Tab` | Switch focus between navigation and main panel |
| `Left` / `Right` | Move between navigation items or change focus |
| `Up` / `Down` | Move through the focused list or view |
| `Enter` | Open the selected item or confirm an action |
| `/` | Filter the current container or service list |
| `r` or `Space` | Refresh the current list or view |
| `Esc` | Close dialogs, close logs, or go back |
| `q` / `Ctrl+C` | Quit |

Container actions:

| Key | Action |
| --- | --- |
| `i` | Inspect container |
| `l` | Follow container logs |
| `r` | Restart container |
| `s` | Stop container |
| `k` | Kill container |
| `t` | Start container |

Service actions:

| Key | Action |
| --- | --- |
| `i` | Inspect service |
| `t` | Show service tasks |
| `s` | Scale service |
| `r` | Remove service |

Inside service inspect, editable rows can be selected with `Enter`. Changed values are staged until saved.

## Development

Run all tests:

```sh
go test ./...
```

Run a focused test:

```sh
go test ./... -run TestName
```

Compile all packages:

```sh
go build ./...
```

Format changed Go files before committing:

```sh
gofmt -w *.go internal/component/*.go
```

Update module requirements after dependency changes:

```sh
go mod tidy
```

## Project Structure

```text
.
├── main.go                 # Program entry point
├── model.go                # Bubble Tea model, update flow, view rendering
├── docker.go               # Docker command/API helpers and log streaming
├── container.go            # Container list, inspect, logs, and actions
├── service.go              # Service list, tasks, inspect, scale, and removal
├── service_inspect.go      # Editable service inspect state
├── internal/component/     # Reusable TUI components
└── main_test.go            # Unit and rendering tests
```

## Notes

This tool executes Docker operations against the daemon configured in the current environment, including container lifecycle changes and service updates. Review the selected item before confirming destructive actions.
