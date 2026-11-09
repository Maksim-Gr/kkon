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

## CI and releases

Pull requests, pushes to `main`, and release tags use the reusable validation
workflow. Docker integration tests for Kafka Connect and Schema Registry run
only on pull requests, avoiding repeats after merging and tagging. Formatting,
static checks, unit tests, release packaging, and vulnerability scanning run on
all three events. A release is published only after its validation jobs pass for
the tagged commit.

Unit tests run on Linux, macOS, and Windows; Linux also enables the race detector:

```bash
go test -short -race -timeout=10m ./...
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

Vulnerability scanning also runs weekly and can be started manually from Actions.
Go comes from `go.mod`; workflows pin linting, packaging, and scanning tools to
exact versions. Dependabot maintains the action commit pins.

Integration tests require a working Docker daemon in CI. Locally they skip when
Docker is unavailable unless strict mode is enabled:

```bash
KKON_REQUIRE_DOCKER=1 go test -count=1 -timeout=25m -v ./tests
```

Strict mode rejects `-short`. Failed tests log container output before cleanup;
CI retains the integration test log as an artifact for seven days.

With GoReleaser v2.20.0 installed, validate all five release targets without
publishing:

```bash
goreleaser check
goreleaser release --snapshot --clean
(cd dist && shasum -a 256 -c ./*_checksums.txt)
```

On Linux amd64, `python3 scripts/verify-release.py` additionally verifies archive
contents and runs the packaged binary to check its version, commit, and build
date. Native unit tests cover macOS and Windows separately.

Published archives and checksum files receive GitHub build provenance attestations.
After downloading a release asset, verify it with the GitHub CLI:

```bash
gh attestation verify ./kkon_VERSION_linux_amd64.tar.gz --repo Maksim-Gr/kkon
```

Replace `VERSION` with the release version. The attestation records where the
asset was built; the published checksum file verifies download integrity.
Attestation generation follows GoReleaser publishing, so an attestation failure
can leave an already published release requiring a workflow rerun. Release runs
are never automatically cancelled by a newer tag or commit.
