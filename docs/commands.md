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
- `set` Prompt for Kafka Connect URL and optional basic auth.
- `show` Print the current config file.

Examples:
```bash
kkon config set

kkon config show
```

## connector

```bash
kkon connector <subcommand>
```

Subcommands:
- `create` Create a connector from predefined templates or from a JSON file.
- `update` Update an existing connector's configuration (shows a before→after diff).
- `delete [name]` Delete a connector (interactive, or pass a name).
- `list` List connectors and interactively show one config.
- `pause [name]` Pause a connector and its tasks.
- `resume [name]` Resume a paused connector.
- `restart [name]` Restart a connector (and, by default, its tasks).
- `health-check` Print connector status summary.
- `backup` Back up all connector configs to JSON files.

Flags:
- `kkon connector create --file, -f` Path to connector JSON config file.
- `kkon connector delete --yes, -y` Skip the confirmation prompt (for scripting).
- `kkon connector restart --include-tasks` Also restart the connector's tasks (default `true`).
- `kkon connector restart --only-failed` Restart only FAILED connector and tasks (default `false`).
- `kkon connector restart --yes, -y` Skip the confirmation prompt (for scripting).
- `kkon connector backup --dir, -o` Directory to save backup files (default `./backup`).

Notes:
- `create` without `--file` opens an interactive prompt with predefined templates.
- `list` prompts you to select a connector and then prints its config.
- `delete`, `pause`, `resume`, and `restart` take an optional connector name — omit it to select interactively.
- `delete` and `restart` accept `--yes, -y` to skip the confirmation prompt (useful in scripts/CI).

Examples:
```bash
kkon connector create

kkon connector create --file ./my-connector.json

kkon connector update

kkon connector delete --connector my-connector

kkon connector list

kkon connector pause my-connector

kkon connector resume my-connector

kkon connector restart my-connector --only-failed

kkon connector health-check

kkon connector backup --dir ./backup
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
