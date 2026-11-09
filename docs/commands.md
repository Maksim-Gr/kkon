# Commands

`kkon` uses subcommands to manage Kafka Connect and Confluent Schema Registry.

## Root

```bash
kkon [flags] <command> [subcommand] [flags]
```

Global flags:
- `-d, --dry-run` Preview mutating commands without executing them. Applies to commands that change Kafka Connect state, publish schemas, or write files (`create`, `update`, `delete`, `pause`, `resume`, `restart`, `backup`, `restore`, `task restart`, `schema publish`, `config set`); read-only commands ignore it.
- `-o, --output` Output format: `text` (default) or `json`. With `json`, read commands print pure JSON to stdout (errors go to stderr). Connector/task mutating commands accept `json` only when fully non-interactive (connector name given, plus `--yes` where a confirmation exists) and print a small result object; combined with `--dry-run`, the same object shape is printed with `"result": "dry-run"` instead of being run. Schema output shapes are described below.

Exit codes:
- `0` Success, user cancel (Ctrl+C or `← Cancel`), or nothing to do (e.g. no connectors found).
- `1` Any failure (API error, invalid input, validation error).

## config

```bash
kkon config <subcommand>
```

Subcommands:
- `set` Prompt for Kafka Connect URL and optional basic auth, then optionally prompt for Schema Registry URL and basic auth too. Use `--schema-registry` to configure only the registry.
- `show` Print the current config file.

Examples:
```bash
kkon config set

kkon config show
```

## schema

These commands use the Confluent Schema Registry API and require only its endpoint.
Configure it with `kkon config set --schema-registry` or `KKON_SCHEMA_REGISTRY_URL`.

```bash
kkon schema list                              # List subjects, sorted by name
kkon schema versions orders-value             # List registered version numbers
kkon schema get orders-value                  # Raw schema content, latest version
kkon schema get orders-value --version 2 --output json
kkon schema check orders-value -f order.avsc
kkon schema publish orders-value -f order.avsc
kkon schema publish orders-value -f order.json --type json --dry-run
kkon schema publish events-value -f event.proto --type protobuf --references refs.json
```

- Subjects identify collections of schema versions; publishing may create one.
- In a terminal, missing subjects open a selection prompt (publishing also allows
  creating a subject). Missing files prompt for a path with tab completion and a
  schema type. JSON mode and scripts require explicit arguments.
- `check` and `publish` accept `--type avro|json|protobuf` (default `avro`). Files
  contain raw schemas, not REST request envelopes. The registry validates their format.
- `check` uses the effective subject/global policy, including transitive checks,
  without registering a version. It prints available incompatibility reasons and
  exits `1` if incompatible. An unknown subject or one with no versions reports
  that compatibility cannot be evaluated. `--dry-run` does not skip checks.
- `publish` registers directly without confirmation. Registration enforces the
  registry's current compatibility policy, which may change after an earlier check.
- `publish --dry-run` prints the subject and request content without sending a
  registration or compatibility request. It does not validate compatibility.
- `get` defaults to `--version latest`; positive version numbers are also accepted.
  Text output is raw schema content. JSON output includes `subject`, `id`, `version`,
  `schemaType`, `schema`, and `references`.
- JSON `list` and `versions` return arrays. `check` returns `is_compatible` and
  `messages`, even on incompatibility; errors go to stderr. Publishing returns
  `subject` and `id`; dry-run returns `subject`, `dry_run: true`, and the schema request.

The optional `--references` file is a JSON array. Each dependency must already be
registered, with its exact reference name and positive version number:

```json
[
  {"name": "common.proto", "subject": "common", "version": 1}
]
```

Avro references use the referenced type's full name, Protobuf references use import
paths, and JSON Schema references use `$ref` names. Dependencies are resolved by
the registry; kkon does not publish them automatically. This release does not
delete schemas or change compatibility policies.

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
