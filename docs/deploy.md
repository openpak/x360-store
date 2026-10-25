# Deploy: x360-store

OpenPak's Xbox 360 store backend: the homebrew catalog and the Live download queue
(Lists, Live service 10). One service, repository and image, like the other `x360-*` services
(shared code from [`x360-common`](../../x360-common) via `replace` + committed `vendor/`; after
changing it run `GOWORK=off go mod vendor` here and commit).

## Configuration, build, release

`X360_STORE_ADDR` (default `:10044`), `X360_STORE_CATALOG` (optional), `X360_GATEWAY_KEY`
(EnvironmentFile `%h/openpak/deploy/x360-gateway-key.env`). Quadlet and Traefik fragment in
`deploy/`. `GOWORK=off go test -mod=vendor ./...`. Tag `vX.Y.Z` and CI publishes
`ghcr.io/openpak/x360-store` (amd64 + arm64). Licensed AGPL-3.0, see LICENSE.
