# bao-rotator

Secret rotation CLI for OpenBao / Vault.

See [docs/tools/bao-rotator.md](../../docs/tools/bao-rotator.md) for full documentation.

## Quick Start

```bash
go build -o bao-rotator ./cmd
./bao-rotator audit kv
./bao-rotator rotate kv myapp/api-key
```
