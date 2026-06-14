# Common workflows

## Initial setup and verify

```bash
kkon config configure

kkon connector list
```

## Create a connector from a JSON file

```bash
kkon connector create --file ./connector.json
```

## Create a connector interactively

```bash
kkon connector create
```

Follow the prompts to fill required fields and optionally submit the config.

## Inspect a connector config

```bash
kkon connector list
```

Pick a connector when prompted to print its config.

## Check connector health

```bash
kkon connector health-check
```

## List tasks and check a task status

```bash
kkon task list --connector my-connector

kkon task get --connector my-connector --id 0
```

## Restart a task

```bash
kkon task restart --connector my-connector --id 0
```

## Backup all connector configs

```bash
kkon config backup --dir ./backups
```

## Dry run a change

```bash
kkon --dry-run task restart --connector my-connector --id 0
```
