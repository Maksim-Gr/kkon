# CHANGELOG

---

## Unreleased

### Added
- Environment variables `KKON_CONNECT_URL`, `KKON_CONNECT_USERNAME`,
  `KKON_CONNECT_PASSWORD`, and `KKON_SCHEMA_REGISTRY_URL` override the config
  file. With `KKON_CONNECT_URL` set, no config file is required and the
  first-run setup is skipped. `kkon config show` prints the effective config
  and lists active overrides.
- `task restart` accepts `--yes/-y` and `-o json` (with `--connector`, `--id`
  and `--yes`); `connector update` accepts the connector name as an argument.

### Changed
- API errors show Kafka Connect's `message` instead of the raw JSON body.
- `task get` ignores `--dry-run`, like the other read-only commands.
- The spinner is not drawn when stderr is not a terminal.
- The plugin wizard masks more credential-like fields (tokens, API keys,
  credentials, `sasl.jaas.config`).
- `create --file` accepts number and boolean config values (sent as strings)
  and validates them like any other file; previously such files skipped
  validation. Nested values, top-level fields other than `name` and `config`,
  and a top-level `name` that differs from the config's `name` are rejected.
- Ctrl+C at any prompt cancels the whole command (exit 0). Previously it
  skipped just that step at restore's overwrite prompt, `config set`'s
  connection test and the create wizard's key-format question. Other prompt
  failures (e.g. a broken terminal) now exit 1 instead of reporting "Canceled".
- Selection lists show 10 entries per page.
- `restore` reports failures as "failed to restore N of M connector(s)", like
  the other bulk commands.

### Fixed
- Connector names containing `#`, `?`, `%` or `/` are now URL-escaped;
  previously requests were truncated or hit the wrong endpoint.
- `config set` no longer drops the saved Schema Registry URL.
- A malformed config file is reported instead of silently re-running setup
  and overwriting it. Without a terminal (or with `-o json`), a missing URL
  fails with a hint to run `kkon config set` or set `KKON_CONNECT_URL`.
- `create --file` with a flat config map wraps it as `{"name","config"}`
  instead of submitting it as-is, which Kafka Connect rejected.
- `health-check` and the create wizard's field list print in sorted order.
- Ctrl+C during the post-update health check takes effect immediately.

---

## v2.0.0 — 2026-07-03

### Added
- `connector create` can now configure a connector from **any plugin installed
  on the cluster**: pick `Custom (choose from installed plugins)` in the wizard
  or pass `--plugin <class>`. The wizard uses the Kafka Connect validate
  endpoint to discover required fields — every required field is prompted for
  (its plugin-defined default, if any, is pre-filled as the suggested answer
  rather than silently accepted), offers server-recommended values as
  choices, and masks password/secret fields.
- Bulk operations: `pause`, `resume`, `restart`, and `delete` now accept
  multiple connector names, `--all`, or an interactive multi-select. Bulk runs
  continue past individual failures, print a per-connector ✓/✗ summary, and
  exit `1` if any operation failed. `pause`/`resume` gained `--yes` (confirmation
  is only asked when targeting more than one connector).

### Changed
- `connector restore` no longer stops at the first failure: every connector in
  the backup is attempted (in sorted order), a per-connector ✓/✗ summary is
  printed, and the command exits `1` if any restore failed. With `-o json` the
  per-connector results are emitted as a JSON array.
- **Breaking for scripts:** commands now exit with code `1` on failure
  (previously failures were printed but the process exited `0`). User cancels
  and no-op runs still exit `0`.
- `--dry-run` is now honored consistently by all mutating commands, including
  `create`, `update`, and `backup`.
- `--output json` now produces clean, pipeable stdout: the spinner writes to
  stderr, and mutating commands either emit a result object (when fully
  non-interactive) or fail fast instead of prompting.
- `connector list --config <name> -o json` prints the raw config JSON, and
  unknown connector names are reported as errors.

### Fixed
- The create wizard now submits the payload shape `POST /connectors` expects
  (`{"name": ..., "config": {...}}`) and prompts for a connector name when the
  template doesn't carry one — previously every template submission was
  rejected by the server.
- `--dry-run` combined with `-o json` now emits a JSON result (or array of
  results) with `"result": "dry-run"` instead of plain text, for `pause`,
  `resume`, `restart`, `delete`, `restore`, and `create --file`.
- `backup` now supports `-o json`, emitting a result object on both the
  dry-run and real paths instead of silently ignoring the flag.

---

## v1.0.1 — 2026-06-12

### Added
- Connector lifecycle commands: `gk connector pause`, `resume`, and `restart`
  (with `--include-tasks` and `--only-failed` flags on restart)
- `gk version` command (and `--version`) reporting the build version, commit, and date
- `--state` filter on `gk connector list` and `gk connector health-check`
- Pre-submit config validation for `create` and `update`, surfacing
  field-level errors before the config is sent

### Changed
- `gk config set` now returns errors through Cobra instead of calling `os.Exit`

### Fixed
- Added unit tests for the Kafka Connect HTTP client and config round-trip

---

## Unreleased

Initial development of **kc**.

### Added
- Kafka Connect REST API client abstraction
- Connector operations:
    - List connectors
    - View connector configuration (raw and JSON)
    - Create connectors from predefined templates
    - Delete connectors
    - Backup connector configurations to timestamped JSON files
- Task operations:
    - List tasks for a connector
    - Get task status
    - Restart a task
- Config operations:
    - Configure Kafka Connect URL
    - Show current configuration
- Interactive CLI prompts for connector/task selection
- Configuration-driven Kafka Connect URL loading

### Changed
- CLI commands reorganized into subdirectories/packages (`cmd/config`, `cmd/connector`, `cmd/task`) for clearer separation

### Fixed
- Configuration file resolution to avoid failures when running from different working directories / build contexts

### Breaking Changes
- Command layout changed due to CLI package reorganization (subcommands moved under `config`, `connector`, `task`)

---

_This project is under active development. Versions and release notes will be added once the first stable release is published._