# kkon

[![Go](https://github.com/Maksim-Gr/kkon/actions/workflows/go.yml/badge.svg)](https://github.com/Maksim-Gr/kkon/actions/workflows/go.yml)
[![Latest Release](https://img.shields.io/github/v/release/Maksim-Gr/kkon?include_prereleases)](https://github.com/Maksim-Gr/kkon/releases)

A CLI for managing Kafka Connect connectors and Confluent Schema Registry schemas.
Operate and back up connectors, browse schemas, check compatibility, and publish
schema versions from your terminal.

![kkon demo: list connectors, inspect status and config, and create a connector from an installed plugin](./assets/demo.gif)

Commands support interactive prompts or explicit arguments for scripts. Use
`--output json` for JSON output and `--dry-run` to preview changes.

See the [documentation](./docs/README.md) for configuration, commands, and workflows.

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

### Environment variables

`KKON_CONNECT_URL`, `KKON_CONNECT_USERNAME`, and `KKON_CONNECT_PASSWORD` override
Connect configuration. Schema Registry uses independent `KKON_SCHEMA_REGISTRY_URL`,
`KKON_SCHEMA_REGISTRY_USERNAME`, and `KKON_SCHEMA_REGISTRY_PASSWORD` overrides.
Set the relevant URL to run without a config file or interactive setup:

```bash
KKON_CONNECT_URL=http://connect:8083 kkon connector health-check --output json
```

## Usage

```bash
kkon --help
```

### Connector commands

```bash
kkon connector list                      # List connectors with status badges
kkon connector create                    # Create from template (RabbitMQ, S3 Sink, JDBC, Debezium Postgres)
kkon connector create --plugin <class>   # Create from any plugin installed on the cluster
kkon connector create -f connector.json  # Create from JSON file
kkon connector update [name]             # Update connector config (shows before→after diff)
kkon connector delete [name...]          # Delete one or more connectors (interactive multi-select, names, or --all)
kkon connector delete my-conn --yes      # Delete without the confirmation prompt (scriptable)
kkon connector pause [name...]           # Pause connectors and their tasks
kkon connector resume [name...]          # Resume paused connectors
kkon connector restart [name...]         # Restart connectors (and their tasks)
kkon connector restart [name] --only-failed     # Restart only FAILED connector and tasks
kkon connector restart --all --yes       # Restart every connector, skipping confirmation
kkon connector health-check              # Show connector and task statuses with error traces
kkon connector plugins                   # List connector plugins installed on the cluster
kkon connector plugins --type sink       # Filter plugins by type (source or sink)
kkon connector backup                    # Back up all connector configs to JSON
kkon connector restore [file]            # Restore connectors from a backup file (interactive if omitted)
```

> `delete`, `pause`, `resume`, and `restart` take optional connector names — omit them to multi-select interactively, or pass `--all` to target every connector. All four accept `--yes/-y` to skip the confirmation prompt (for scripting); `pause`/`resume` only confirm when targeting more than one connector. Operating on multiple connectors continues past individual failures, prints a per-connector ✓/✗ summary, and exits `1` if any operation failed. `restart` restarts tasks by default (`--include-tasks`); use `--only-failed` to restart only failed connectors/tasks.

### Task commands

```bash
kkon task list -c <name>      # List tasks for a connector
kkon task get  -c <name>      # Get task status
kkon task restart -c <name>   # Restart a task
```

### Schema Registry commands

Configure the registry with `kkon config set --schema-registry` or set
`KKON_SCHEMA_REGISTRY_URL`. No Kafka Connect endpoint is required.

```bash
kkon schema list
kkon schema versions orders-value
kkon schema get orders-value --version latest
kkon schema check orders-value -f order.avsc
kkon schema publish orders-value -f order.avsc
kkon schema publish events-value -f event.proto --type protobuf --references refs.json
```

Schema types are `avro` (default), `json` (JSON Schema), and `protobuf`.
Subjects name collections of schema versions. Omit a subject or file in a terminal
to select it interactively. Explicit arguments publish directly without confirmation.
Checks use the registry's configured compatibility policy, including transitive
checks, and exit `1` if incompatible or if the subject has no registered versions.
Publishing may create a subject. Use `--dry-run` to preview without registering.

See [schema commands](./docs/commands.md#schema) for references and JSON output.

### Config commands

```bash
kkon config set               # Set Kafka Connect URL and credentials
kkon config show              # Display current configuration
```

### Global flags

```bash
--dry-run, -d        Preview a mutating command without executing it (read-only commands ignore it)
--output, -o <fmt>   Output format: text (default) or json
```

Exit codes: `0` for success, user cancel, or nothing to do (e.g. no connectors found); `1` for any failure.

Create and update previews mask credential fields. An update exits `1` if its
result cannot be verified or the connector does not return to RUNNING, including
when the previous config is restored. Empty bulk operations emit `[]` in JSON mode.

## Backup and restore

The `backup` command retrieves all connector configurations from the Kafka Connect cluster and stores them in a timestamped JSON file:

```bash
kkon connector backup --dir ./backup
```

Restore them with:

```bash
kkon connector restore ./backup/config_20240101_120000.json
```

Backups use unique filenames and owner-only permissions. A complete backup is
published atomically using a hard link, so the backup directory must be on a
filesystem that supports hard links. Failed writes do not replace previous backups.

Run `kkon connector restore` with no argument to pick a backup file interactively from the backup directory. Existing connectors are only overwritten after confirmation (use `--yes` to skip prompts, or `--dry-run` to preview). Restoring continues past individual failures, prints a per-connector ✓/✗ summary, and exits `1` if any connector failed to restore.

Without `--yes`, restore aborts if it cannot list existing connectors to check
for overwrites.

## Contributing

- Check open issues before submitting a duplicate
- Fork the repository and open a pull request against `main`
- Follow the existing code style (`gofmt`, `go vet`)
- Integration tests require Docker; run `make test`

## References

- [Kafka Connect REST API](https://docs.confluent.io/platform/current/connect/references/restapi.html)
