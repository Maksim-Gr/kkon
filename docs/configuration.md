# Configuration

`kkon` needs the endpoint and optional basic-auth credentials for the service used
by a command. Schema commands require only Schema Registry; connector and task
commands require only Kafka Connect.

## Where configuration lives

`kkon` stores its config at:

- `~/.config/kkon/config.yaml`

You can create or update it via the interactive command:

```bash
./kkon config set
./kkon config set --schema-registry # Configure only Schema Registry
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
  username: ""
  password: ""
```

Notes:
- `kafkaConnect.username`/`password` are optional. Leave them empty for no auth.
- If you enter a URL without a scheme, `kkon` assumes `http://`.
- The `schemaRegistry` section is optional for connector operations. Configure it
  independently with `kkon config set --schema-registry`, or opt in during the
  regular wizard. Its URL also prefills the converter URL when
  `kkon connector create` asks whether to use Avro/Protobuf/JSON Schema
  converters.
- Registry credentials are independent of Connect credentials. For Confluent Cloud,
  use the Schema Registry API key as username and API secret as password. These
  credentials authenticate schema commands; they are not copied into connector configs.
- HTTPS uses standard certificate verification. OAuth and custom client certificates
  are not supported by schema commands in this release.

## Environment variables

Environment variables override the matching config file values, which makes
`kkon` usable in CI without a config file or the interactive setup:

| Variable | Overrides |
|----------|-----------|
| `KKON_CONNECT_URL` | `kafkaConnect.url` |
| `KKON_CONNECT_USERNAME` | `kafkaConnect.username` |
| `KKON_CONNECT_PASSWORD` | `kafkaConnect.password` |
| `KKON_SCHEMA_REGISTRY_URL` | `schemaRegistry.url` |
| `KKON_SCHEMA_REGISTRY_USERNAME` | `schemaRegistry.username` |
| `KKON_SCHEMA_REGISTRY_PASSWORD` | `schemaRegistry.password` |

Empty variables are ignored. When `KKON_CONNECT_URL` is set, no config file is
needed for connector commands and the first-run setup is skipped. Likewise,
`KKON_SCHEMA_REGISTRY_URL` is sufficient for schema commands. URLs without a
scheme get `http://`.

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
Both Connect and Schema Registry passwords are masked.

## Dry run

Some commands support `--dry-run` (global flag) to show what would happen without making changes:

```bash
./kkon --dry-run config set
```
