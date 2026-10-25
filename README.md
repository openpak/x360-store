# Xbox 360 Store Server

OpenPak's Xbox 360 Store Server.

| | |
| --- | --- |
| Network | Xbox 360 |
| Role | Homebrew catalog and Live download queue (Lists, Live service 10) |
| Depends on | [`x360-common`](https://github.com/openpak/x360-common) (shared Go library, vendored) |
| Image | `ghcr.io/openpak/x360-store` |
| Used by | OpenPak website/app (catalog), XAM through the security gateway (download queue) |
| Licence | AGPL-3.0-only |

## What it serves

| Path | What |
| --- | --- |
| `GET /v1/catalog`, `GET /healthz` | Homebrew catalog for the website/app; health probe |
| `POST /Lists/*.ashx` (XRL 0x1201, 0x1202, 0x1206-0x1208) | Live download queue for XAM, through the security gateway |
| `POST /v1/admin/queue`, `DELETE /v1/admin/queue` | Operator endpoints (Bearer `X360_STORE_ADMIN_KEY`): queue an item for one user, drop a queue |

## Run

```sh
GOWORK=off go run -mod=vendor ./cmd/x360-store
```

## Docs

- [Protocol](docs/protocol.md)
- [Evidence](docs/evidence.md)
- [Deploy](docs/deploy.md)
- [Notes](docs/notes.md)

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
