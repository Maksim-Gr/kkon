# CHANGELOG

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