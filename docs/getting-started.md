# Getting started

## Prerequisites

- Go installed (if building from source)
- Access to a Kafka Connect REST API or Confluent Schema Registry endpoint

## Build from source

From the repository root:

```bash
go build -o kkon .
```

## Run

```bash
./kkon --help
```

## First-time setup

`kkon` needs a Kafka Connect URL (and optional basic auth). You can configure it explicitly:

```bash
./kkon config set
```

If no configuration exists, `kkon` will prompt you automatically on the next run.

For Schema Registry alone, configure its independent endpoint and credentials:

```bash
./kkon config set --schema-registry
./kkon schema list
```

Schema commands do not need a Kafka Connect endpoint. In scripts, set
`KKON_SCHEMA_REGISTRY_URL` and optional `KKON_SCHEMA_REGISTRY_USERNAME` /
`KKON_SCHEMA_REGISTRY_PASSWORD` instead of using the wizard.

## Quick verification

```bash
./kkon connector list
```

If you get an error, jump to [Troubleshooting](./troubleshooting.md).
