# Commands

`kkon` uses a small set of subcommands to manage Kafka Connect.

## Root

```bash
kkon [flags] <command> [subcommand] [flags]
```

Global flags:
- `-d, --dry-run` Preview mutating commands without executing them. Applies to commands that change Kafka Connect state or write files (`create`, `update`, `delete`, `pause`, `resume`, `restart`, `backup`, `restore`, `task restart`, `config set`); read-only commands ignore it.
- `-o, --output` Output format: `text` (default) or `json`. With `json`, read commands print pure JSON to stdout (errors go to stderr). Mutating commands accept `json` only when fully non-interactive (connector name given, plus `--yes` where a confirmation exists) and print a small result object; combined with `--dry-run`, the same object shape is printed with `"result": "dry-run"` instead of being run.

Exit codes:
- `0` Success, user cancel (Ctrl+C or `← Cancel`), or nothing to do (e.g. no connectors found).
- `1` Any failure (API error, invalid input, validation error).

## config

```bash
kkon config <subcommand>
```

Subcommands:
- `set` Prompt for Kafka Connect URL and optional basic auth, then optionally prompt for Schema Registry URL and basic auth too.
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
- `create` Create a connector from predefined templates, from any installed plugin (`--plugin <class>` or the `Custom` wizard option), or from a JSON file.
- `update [name]` Update an existing connector's configuration (shows a before→after diff).
- `delete [name...]` Delete one or more connectors (multi-select, names, or `--all`).
- `list` List connectors and interactively show one config.
- `pause [name...]` Pause one or more connectors and their tasks.
- `resume [name...]` Resume one or more paused connectors.
- `restart [name...]` Restart one or more connectors (and, by default, their tasks).
- `health-check` Print connector status summary.
- `plugins` List connector plugin classes installed on the cluster.
- `backup` Back up all connector configs to a timestamped JSON file.
- `restore [file]` Re-create connectors from a backup file (pick one interactively if omitted).

Flags:
- `kkon connector create --file, -f` Path to connector JSON config file.
- `kkon connector delete --yes, -y` Skip the confirmation prompt (for scripting).
- `kkon connector restart --include-tasks` Also restart the connector's tasks (default `true`).
- `kkon connector restart --only-failed` Restart only FAILED connector and tasks (default `false`).
- `kkon connector restart --yes, -y` Skip the confirmation prompt (for scripting).
- `kkon connector health-check --fail-on-unhealthy` Exit `1` if any listed connector or task is not RUNNING (for CI/monitoring).
- `kkon connector backup --dir` Directory to save backup files (default `./backup`).
- `kkon connector restore --dir` Directory to look for backup files (default `./backup`).
- `kkon connector restore --yes, -y` Overwrite existing connectors without confirmation.

Notes:
- `create` without `--file` opens an interactive prompt with predefined templates.
- `create`'s review step asks whether to use Avro/Protobuf/JSON Schema (Confluent Schema Registry converters) for the connector's value (and optionally key), prefilling the URL from the configured Schema Registry if there is one.
- `list` prompts you to select a connector and then prints its config.
- `delete`, `pause`, `resume`, and `restart` take optional connector names — omit them to multi-select interactively, or pass `--all` to target every connector.
- All four accept `--yes, -y` to skip the confirmation prompt (useful in scripts/CI). `pause` and `resume` only confirm when targeting more than one connector.
- Operating on multiple connectors continues past individual failures, prints a per-connector ✓/✗ summary, and exits `1` if any operation failed.

Examples:
```bash
kkon connector create

kkon connector create --file ./my-connector.json

kkon connector update

kkon connector delete my-connector

kkon connector list

kkon connector pause my-connector

kkon connector resume my-connector

kkon connector restart my-connector --only-failed

kkon connector pause --all --yes

kkon connector restart connector-a connector-b --yes

kkon connector health-check

kkon connector health-check --fail-on-unhealthy

kkon connector plugins --type source

kkon connector backup --dir ./backup

kkon connector restore ./backup/config_20260710_120000.json
```

## task

```bash
kkon task <subcommand> [flags]
```

Subcommands:
- `list` List task IDs for a connector.
- `get` Get status for a single task.
- `restart` Restart a single task (with confirmation; `--yes, -y` skips it).

Flags:
- `--connector, -c` Connector name (optional; prompts if missing).
- `--id, -i` Task id (integer; optional; prompts if missing).

Examples:
```bash
kkon task list --connector my-connector

kkon task get --connector my-connector --id 0

kkon task restart --connector my-connector --id 1

kkon task restart --connector my-connector --id 1 --yes --output json
```
