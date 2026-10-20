# x360-store

OpenPak's Xbox 360 store backend: the homebrew catalog and the Live download queue
(Lists, Live service 10). One service, repository and image, like the other `x360-*` services
(shared code from [`x360-common`](../x360-common) via `replace` + committed `vendor/`; after
changing it run `GOWORK=off go mod vendor` here and commit).

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

## The catalog is empty, on purpose

OpenPak lists only packages it can show to be redistributable: every item needs an SPDX
`license` and a public `source` repository, and the loader rejects anything else. No Xbox 360
homebrew package meeting that bar has been added yet, so the catalog answers
`{"network":"xbox360","count":0,"items":[]}`. Nothing copyrighted, extracted from firmware or
taken from Microsoft's store is bundled or listed. Switch packages are not usable on the 360.

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

## Why the dashboard has no store tile (2026-10-20 findings)

The stock 17559 dashboard does not contain its store. `EcNavMarketplaceScriptAction`
(`browseui`, `livetitlelist`, `titlecontentlist`, `titlecontentitem`, `xboxoriginalsall`) and
`EcNavToURI hub:root` all open `Dash.MP.ContentExplorer.lex`; purchases open
`Dash.MP.Purchase.lex`; the Guide's download list is `Guide.MP.Download.xex`. Those modules
(and `Dash.MP.Core.xex`, `Dash.Social.lex`, `Dash.OnlineCommon.lex`, `Dash.Search.lex`,
`Guide.MP.*.xex`) are not in the system update. XAM downloads them after a Live sign-in from
`<UpdateUriRoot1>/fffe07d1/manifests/<SOMRevision>/system.online.manifest` and
`.../files/...` into the System Aux partition (`\sap\online`). Only their localized resource
`.xzp` files are in the update. OpenPak cannot redistribute Microsoft's modules, so on Wharf
(which has none) the stock store entry points do nothing. A stock-store tile would be dead
on every console without those cached modules. The native store needs either:

1. replacement OpenPak modules with the same names (XEX `.lex` packages carrying our own
   Lua/XUI store, talking to this service). This is a new client project. The modules must run unsigned,
   so they work only on Wharf and modified consoles; or
2. consoles that already cached Microsoft's modules from an earlier Live sign-in, plus
   OpenPak implementations of the backends those modules call. The dashboard's own Lua shows
   EDS (`{LiveHive:EDSUriRoot}/media/{locale}/...`), commerce
   (`LiveServiceUrl.commerce`) and list (`ListServiceUriRoot`) templates. The browse queries
   live inside the missing modules, so they cannot be read from firmware we have.

## Download and install without the store UI

XAM itself runs the queue and the background downloads (`XamBackgroundDownload*`, a 3-second
XAM task). The dashboard enables it at boot (SetMode 1). At every Live sign-in, and then hourly
(LiveHive `SSDQ*PollFrequencyInSeconds`), it asks 0x1202. Verified in Wharf on 2026-10-20 with
service 10 in the KDC D7 list: the 32-byte 0x1202 request arrives with the user's XUID. For a
picked item XAM then:

1. calls 0x914 = 0x931 FindMediaInstanceUrls (Live service 9, not served yet). The reply needs
   exactly one entry with a 16-byte content key, a 284-byte RSA-2048 public key blob and up to
   10 plain `http://` URLs;
2. downloads the URL. The body must be a signed, RC4-encrypted Microsoft Cabinet whose
   reserve is verified with that RSA key. The server chooses both keys, so OpenPak can produce
   valid files. A CAB writer is still to be built;
3. extracts a `LIVE` STFS package, verifies it with XeKeysVerifyRSASignature key 1 (Wharf: the
   OpenPak content signer via `--xsigner_public_key`; retail consoles: Microsoft's key, so
   modified kernels only), installs it through XContent, and deletes the item (0x1206).
   An STFS LIVE writer is still to be built.

## Configuration, build, release

`X360_STORE_ADDR` (default `:10044`), `X360_STORE_CATALOG` (optional), `X360_GATEWAY_KEY`
(EnvironmentFile `%h/openpak/deploy/x360-gateway-key.env`). Quadlet and Traefik fragment in
`deploy/`. `GOWORK=off go test -mod=vendor ./...`. Tag `vX.Y.Z` and CI publishes
`ghcr.io/openpak/x360-store` (amd64 + arm64). Licensed AGPL-3.0, see LICENSE.
