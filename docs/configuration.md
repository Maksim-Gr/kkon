# Configuration

`kkon` needs to know how to reach your Kafka Connect REST API and (optionally) basic auth credentials.

## Where configuration lives

`kkon` stores its config at:

- `~/.kkon/config.yaml`

You can create or update it via the interactive command:

```bash
./kkon config configure
```

## Config format

`config.yaml` is YAML and follows this structure:

```yaml
kafkaConnect:
  url: http://localhost:8083
  username: ""
  password: ""
```

Notes:
- `username`/`password` are optional. Leave them empty for no auth.
- If you enter a URL without a scheme, `kkon` assumes `http://`.

## View current config

```bash
./kkon config show-config
```

## Dry run

Some commands support `--dry-run` (global flag) to show what would happen without making changes:

```bash
./kkon --dry-run config configure
```
