# Development

## Build

```bash
go build -o kkon .
```

## Run locally

```bash
./kkon --help
```

## Tests

```bash
go test ./...
```

## Lint and static checks

```bash
golangci-lint run
staticcheck -tests ./...
```

## Format

```bash
gofmt -s -w .
```
