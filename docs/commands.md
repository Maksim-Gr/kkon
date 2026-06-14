# Commands

`kkon` uses a small set of subcommands to manage Kafka Connect.

## Root

```bash
kkon [flags] <command> [subcommand] [flags]
```

Global flags:
- `-d, --dry-run` Run without making changes (supported by some commands).
- `-t, --toggle` Placeholder flag (currently unused).

## config

```bash
kkon config <subcommand>
```

Subcommands:
- `configure` Prompt for Kafka Connect URL and optional basic auth.
- `backup` Dump connector configs to a JSON file.
- `show-config` Print the current config file.

Flags:
- `kkon config backup --dir, -o` Directory to save backup files (default: `./backup`).

Examples:
```bash
kkon config configure

kkon config backup --dir ./backups

kkon config show-config
```

## connector

```bash
kkon connector <subcommand>
```

Subcommands:
- `create` Create a connector from predefined templates or from a JSON file.
- `delete` Delete a connector by name.
- `list` List connectors and interactively show one config.
- `health-check` Print connector status summary.

Flags:
- `kkon connector create --file, -f` Path to connector JSON config file.
- `kkon connector delete --connector, -c` Connector name (required).

Notes:
- `create` without `--file` opens an interactive prompt with predefined templates.
- `list` prompts you to select a connector and then prints its config.

Examples:
```bash
kkon connector create

kkon connector create --file ./my-connector.json

kkon connector delete --connector my-connector

kkon connector list

kkon connector health-check
```

## task

```bash
kkon task <subcommand> [flags]
```

Subcommands:
- `list` List task IDs for a connector.
- `get` Get status for a single task.
- `restart` Restart a single task (with confirmation).

Flags:
- `--connector, -c` Connector name (optional; prompts if missing).
- `--id, -i` Task id (integer; optional; prompts if missing).

Examples:
```bash
kkon task list --connector my-connector

kkon task get --connector my-connector --id 0

kkon task restart --connector my-connector --id 1
```
