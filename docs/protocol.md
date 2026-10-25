# Protocol: x360-store

## What it serves

| Path | Who calls it | Reply |
| --- | --- | --- |
| `GET /v1/catalog[?kind=game\|app]` | OpenPak website/app, internal network | JSON `{network, count, items}` |
| `POST /Lists/QueryDownloadQueues.ashx` (XRL 0x1202) | XAM, through the security gateway | no queue (4 zero bytes) or the user's one queue |
| `POST /Lists/QueryDownloadItems.ashx` (0x1208) | XAM | queued items (item XML) |
| `POST /Lists/ModifyDownloadQueue.ashx` (0x1201) | XAM binds the queue to its console | new version |
| `POST /Lists/DeleteDownloadItems.ashx` (0x1206) | XAM after an install (or for an invalid item) | new version |
| `POST /Lists/ModifyDownloadItems.ashx` (0x1207) | XAM records a failed/paused item | new version |
| other `/Lists/*.ashx` | XAM | 404, logged |
| `POST /v1/admin/queue`, `DELETE /v1/admin/queue?xuid=` | operator (Bearer `X360_STORE_ADMIN_KEY`), internal network | queue an item for one user / drop the queue |
| `GET /healthz` | probes | 200 |

Wire layouts come from XAM 17559's XRL schema table (decoded with the Wharf `xrlcodec.py`),
behaviour from XAM's queue manager (Wharf notes `store-re/download-queue-re.md`). Little-endian,
no padding. 0x1202 replies `u16, u16 count, {u32, u32 queueId, u64 version, u32 itemCount,
u8 nameLen, u16 propsLen, "DownloadQueue", props}`; props is
`<RootNode><dest>consoleId</dest><consoleRegionMask>n</consoleRegionMask></RootNode>`, dest 0 makes
the console bind the queue with 0x1201. 0x1208 replies `u16 total, u64 version, u16 count,
{GUID itemId, u16 1000, u16 len, item XML}`; the version must match the queue's. Mutations reply
with the new u64 version. Queues live in memory (a restart empties them). The handlers read
the gateway's identity headers only through `wire.Guard` (`X360_GATEWAY_KEY`).

## Catalog file

Catalog file (`X360_STORE_CATALOG`, optional):

```json
{"items": [{"id": "example", "name": "Example", "kind": "app", "title_id": "4F50XXXX",
  "package": "CON", "size": 123456, "sha1": "<40 hex>", "url": "https://...",
  "license": "MIT", "source": "https://github.com/...", "description": "..."}]}
```

### What a 360 package must be

- Container: STFS. `CON` packages are signed with a console certificate, `LIVE` and `PIRS`
  with Microsoft keys. Homebrew is normally shipped as `CON`.
- Content type in the STFS header decides where it lands under
  `Content\<profile or 0000000000000000>\<title id>\<content type>\`; games and apps are
  usually Arcade Title (`000D0000`) or Installed Game (`00004000`) packages.
- The XEX inside must be signed with Microsoft's retail key to run on a stock console.
  Homebrew XEX files are not, so they run only on consoles whose kernel accepts unsigned
  executables and packages (RGH/JTAG-class modifications). There is no trust patch OpenPak can
  apply to a retail console from the network side; this is the same boundary as on other
  platforms ("no game patches", the console itself must accept the code).
- Emulators (Wharf) do not verify these signatures.
