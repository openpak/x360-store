# Changelog

- 2026-11-04: v0.4.0: Account erasure (card #120). `DELETE /internal/accounts/{account_id}/data` with `Authorization: Bearer $ACCOUNT_ERASE_KEY` on a separate listener `ERASE_ADDR` (default `:9099`, container network only, never published; not served while the key is unset). Reply `{"service":"x360-store","rows":N}`; one `account erased` log line per erasure. Deletes the account's download queue (`queue/<xuid>`). New optional env `X360_XUACS_INTERNAL_URL` and `X360_XUACS_INTERNAL_KEY` (to find the XUIDs); without them the endpoint is not served.

- 2026-11-03: v0.3.0: download queues move to Postgres (schema `x360_store` in database `x360`, `X360_STORE_DATABASE_URL`,
  required: the service refuses to start without it); a restart no longer empties them.

- 2026-10-20: v0.2.1: parse 0x1207 items as GUID + u16 length + XML (no flags word); log
  FindMediaInstanceUrls (0x930/0x931) requests, still unserved (404).

- 2026-10-20: v0.2.0: per-user download queues (0x1202/0x1208/0x1201/0x1206/0x1207, in memory)
  and an operator endpoint to queue items. XAM's sign-in query was verified in Wharf.

- 2026-10-20: v0.1.0 first release: the homebrew catalog (`GET /v1/catalog`, empty until a
  redistributable package is added) and empty replies for the Lists download-queue queries
  (XRL 0x1202 QueryDownloadQueues, 0x1208 QueryDownloadItems), behind the gateway key guard.
