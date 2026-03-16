# axon-eval

Evaluation framework for running scenario plans against a live service cluster.

## Build & Test

```bash
go test ./...
go vet ./...
```

## Key Files

- `eval.go` — core evaluation engine
- `client.go` — HTTP client for interacting with services under test
- `doc.go` — package documentation
