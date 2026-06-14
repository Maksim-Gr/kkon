# Getting started

## Prerequisites

- Go installed (if building from source)
- Access to a Kafka Connect cluster (its REST API URL)

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
./kkon config configure
```

If no configuration exists, `kkon` will prompt you automatically on the next run.

## Quick verification

```bash
./kkon connector list
```

If you get an error, jump to [Troubleshooting](./troubleshooting.md).
