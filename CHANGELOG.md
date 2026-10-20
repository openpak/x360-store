# Changelog

- 2026-10-20: v0.2.1: parse 0x1207 items as GUID + u16 length + XML (no flags word); log
  FindMediaInstanceUrls (0x930/0x931) requests, still unserved (404).

- 2026-10-20: v0.2.0: per-user download queues (0x1202/0x1208/0x1201/0x1206/0x1207, in memory)
  and an operator endpoint to queue items. XAM's sign-in query was verified in Wharf.

- 2026-10-20: v0.1.0 first release: the homebrew catalog (`GET /v1/catalog`, empty until a
  redistributable package is added) and empty replies for the Lists download-queue queries
  (XRL 0x1202 QueryDownloadQueues, 0x1208 QueryDownloadItems), behind the gateway key guard.
