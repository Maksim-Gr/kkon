# gk CLI

[![Go](https://github.com/Maksim-Gr/gokafkaconnect/actions/workflows/go.yml/badge.svg)](https://github.com/Maksim-Gr/gokafkaconnect/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Maksim-Gr/gokafkaconnect)](https://goreportcard.com/report/github.com/Maksim-Gr/gokafkaconnect)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Maksim-Gr/gokafkaconnect)](https://github.com/Maksim-Gr/gokafkaconnect/blob/main/go.mod)
[![Latest Release](https://img.shields.io/github/v/release/Maksim-Gr/gokafkaconnect?include_prereleases)](https://github.com/Maksim-Gr/gokafkaconnect/releases)

---

A command-line interface for managing Kafka Connect connectors via the Kafka Connect REST API.
`gk` focuses on providing a fast, simple, and interactive CLI experience for day-to-day connector operations.

---

## Overview

`gk` is a Go-based CLI tool designed to interact with Kafka Connect clusters.
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

Download the latest binary for your platform from the [Releases](https://github.com/Maksim-Gr/gokafkaconnect/releases) page, then make it executable:

```bash
chmod +x gk
mv gk /usr/local/bin/gk
```

### Build from source

```bash
git clone https://github.com/Maksim-Gr/gokafkaconnect.git
cd gokafkaconnect
go build -o gk
```

---

## Configuration

On first run `gk` will prompt you to configure a Kafka Connect endpoint.
You can also run configuration manually at any time:

```bash
gk config set
```

Config file location:

| Platform | Path |
|----------|------|
| Linux / macOS | `~/.config/gokafkaconnect/config.yaml` |
| Windows | `%USERPROFILE%\.config\gokafkaconnect\config.yaml` |

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
gk --help
```

### Connector commands

```bash
gk connector list                      # List connectors with status badges
gk connector create                    # Create from template (RabbitMQ, S3 Sink, JDBC, Debezium Postgres)
gk connector create -f connector.json  # Create from JSON file
gk connector update                    # Update connector config (shows before→after diff)
gk connector delete                    # Delete a connector (interactive)
gk connector health-check              # Show connector and task statuses with error traces
```

### Task commands

```bash
gk task list -c <name>      # List tasks for a connector
gk task get  -c <name>      # Get task status
gk task restart -c <name>   # Restart a task
```

### Config commands

```bash
gk config set               # Set Kafka Connect URL and credentials
gk config show              # Display current configuration
gk connector backup         # Backup all connector configs to JSON
gk connector backup --dir ./backup
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
gk connector backup --dir ./backup
```

This allows connector configurations to be versioned, reviewed, or restored later.

---

## Roadmap

- Additional connector templates
- Table output format (`--output table`)

---

## Project Status

`gk` is functional and actively used for connector lifecycle management.
It is currently in pre-release (`v0.x`) while additional connector templates and API features are being added. Breaking changes may occur before `v1.0.0`.

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
