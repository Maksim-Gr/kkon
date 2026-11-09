# Documentation

This folder contains user and developer documentation for the `kkon` CLI.

## Contents

- [Getting started](./getting-started.md)
- [Configuration](./configuration.md)
- [Commands](./commands.md)
- [Common workflows](./workflows.md)
- [Troubleshooting](./troubleshooting.md)
- [Development](./development.md)

## What is `kkon`?

`kkon` manages Kafka Connect connectors and schemas through the Confluent Schema
Registry API. Each service can be configured and used independently.

Typical tasks:
- list connectors
- inspect configs
- create/delete connectors
- back up connector configs
- check connector/task status and restart tasks
- browse schema subjects and versions, check compatibility, and publish schemas
