# kkon CLI

[![Go](https://github.com/Maksim-Gr/kkon/actions/workflows/go.yml/badge.svg)](https://github.com/Maksim-Gr/kkon/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Maksim-Gr/kkon)](https://goreportcard.com/report/github.com/Maksim-Gr/kkon)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Maksim-Gr/kkon)](https://github.com/Maksim-Gr/kkon/blob/main/go.mod)
[![Latest Release](https://img.shields.io/github/v/release/Maksim-Gr/kkon?include_prereleases)](https://github.com/Maksim-Gr/kkon/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/Maksim-Gr/kkon.svg)](https://pkg.go.dev/github.com/Maksim-Gr/kkon)

---

A command-line interface for managing Kafka Connect connectors via the Kafka Connect REST API.
`kkon` focuses on providing a fast, simple, and interactive CLI experience for day-to-day connector operations.

---

## Overview

`kkon` is a Go-based CLI tool designed to interact with Kafka Connect clusters.
It creates a lightweight client for the Kafka Connect REST API and exposes common connector management operations through an intuitive command-line interface.

The tool is intended for developers and operators who want a straightforward way to list, inspect, back up, create, and delete connectors without manually interacting with REST endpoints.

---

## Features

- List running Kafka Connect connectors with live status badges (RUNNING / FAILED / PAUSED)
- View connector configurations
- Create connectors from predefined templates (RabbitMQ, S3 Sink, JDBC, Debezium Postgres)
- Delete existing connectors
- Update connector configuration with a before→after diff view
- Back up connector configurations to JSON files
- Health-check with per-task error trace preview for failed tasks
- Interactive CLI prompts (arrow-key navigation, cancel option on every prompt)
- Connection test after saving credentials
- Basic auth support
- Simple configuration-driven setup
- JSON output for all read commands (`--output json`)

---

## Installation

### Download a release (recommended)

Download the latest binary for your platform from the [Releases](https://github.com/Maksim-Gr/kkon/releases) page, then make it executable:

```bash
chmod +x kkon
mv kkon /usr/local/bin/kkon
```

### Install with Go

```bash
go install github.com/Maksim-Gr/kkon@latest
```

### Build from source

```bash
git clone https://github.com/Maksim-Gr/kkon.git
cd kkon
go build -o kkon
```

---

## Configuration

On first run `kkon` will prompt you to configure a Kafka Connect endpoint.
You can also run configuration manually at any time:

```bash
kkon config set
```

Config file location:

| Platform | Path |
|----------|------|
| Linux / macOS | `~/.config/kkon/config.yaml` |
| Windows | `%USERPROFILE%\.config\kkon\config.yaml` |

Example config:

```yaml
kafkaConnect:
  url: http://localhost:8083
  username: ""
  password: ""
```

---

## Usage

```bash
kkon --help
```

### Connector commands

```bash
kkon connector list                      # List connectors with status badges
kkon connector create                    # Create from template (RabbitMQ, S3 Sink, JDBC, Debezium Postgres)
kkon connector create -f connector.json  # Create from JSON file
kkon connector update                    # Update connector config (shows before→after diff)
kkon connector delete                    # Delete a connector (interactive)
kkon connector pause [name]              # Pause a connector and its tasks
kkon connector resume [name]             # Resume a paused connector
kkon connector restart [name]            # Restart a connector (and its tasks)
kkon connector restart [name] --only-failed     # Restart only FAILED connector and tasks
kkon connector health-check              # Show connector and task statuses with error traces
```

> `pause`, `resume`, and `restart` take an optional connector name — omit it to pick interactively. `restart` restarts tasks by default (`--include-tasks`, on by default); use `--only-failed` to restart only failed connectors/tasks.

### Task commands

```bash
kkon task list -c <name>      # List tasks for a connector
kkon task get  -c <name>      # Get task status
kkon task restart -c <name>   # Restart a task
```

### Config commands

```bash
kkon config set               # Set Kafka Connect URL and credentials
kkon config show              # Display current configuration
kkon connector backup         # Backup all connector configs to JSON
kkon connector backup --dir ./backup
```

### Global flags

```bash
--dry-run, -d        Preview actions without making any API calls
--output, -o <fmt>   Output format: text (default) or json
```

---

## Backup Example

The `backup` command retrieves all connector configurations from the Kafka Connect cluster and stores them in a timestamped JSON file:

```bash
kkon connector backup --dir ./backup
```

This allows connector configurations to be versioned, reviewed, or restored later.

---

## Roadmap

- Additional connector templates
- Table output format (`--output table`)

---

## Project Status

`kkon` is stable and actively used for connector lifecycle management.
Releases follow semantic versioning (`v1.x`); new connector templates and API features continue to be added in backward-compatible minor releases.

---

## Contributing

Contributions, bug reports, and feature requests are welcome.

- Check open issues before submitting a duplicate
- Fork the repository and open a pull request against `main`
- Follow the existing code style (`gofmt`, `go vet`)
- Integration tests require Docker; run `make test`

---

## References

- Kafka Connect REST API documentation:
  https://docs.confluent.io/platform/current/connect/references/restapi.html
