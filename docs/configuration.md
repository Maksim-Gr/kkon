# Configuration

`kkon` needs to know how to reach your Kafka Connect REST API and (optionally) basic auth credentials.

## Where configuration lives

`kkon` stores its config at:

- `~/.config/kkon/config.yaml`

You can create or update it via the interactive command:

```bash
./kkon config set
```

## Config format

`config.yaml` is YAML and follows this structure:

```yaml
kafkaConnect:
  url: http://localhost:8083
  username: ""
  password: ""
schemaRegistry:
  url: http://localhost:8081
```

Notes:
- `kafkaConnect.username`/`password` are optional. Leave them empty for no auth.
- If you enter a URL without a scheme, `kkon` assumes `http://`.
- The `schemaRegistry` section is optional — `kkon config set` only prompts
  for it if you opt in. It's used to prefill the Schema Registry URL when
  `kkon connector create` asks whether to use Avro/Protobuf/JSON Schema
  converters.

## Environment variables

Environment variables override the matching config file values, which makes
`kkon` usable in CI without a config file or the interactive setup:

| Variable | Overrides |
|----------|-----------|
| `KKON_CONNECT_URL` | `kafkaConnect.url` |
| `KKON_CONNECT_USERNAME` | `kafkaConnect.username` |
| `KKON_CONNECT_PASSWORD` | `kafkaConnect.password` |
| `KKON_SCHEMA_REGISTRY_URL` | `schemaRegistry.url` |

Empty variables are ignored. When `KKON_CONNECT_URL` is set, no config file is
needed and the first-run setup is skipped. URLs without a scheme get `http://`.

```bash
KKON_CONNECT_URL=http://connect:8083 kkon connector health-check --output json
```

`kkon config set` only edits the config file; environment variables are never
written to it.

## View current config

```bash
./kkon config show
```

This prints the effective configuration, including any environment variable
overrides (listed on an `Overridden by env:` line).

## Dry run

Some commands support `--dry-run` (global flag) to show what would happen without making changes:

```bash
./kkon --dry-run config set
```
