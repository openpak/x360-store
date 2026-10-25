# Evidence: x360-store

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

1. claims it (0x1207), resolves the install path, then calls 0x914 = 0x931
   FindMediaInstanceUrls (Live service 9). Verified in Wharf with a test item on 2026-10-20:
   the chain 0x1202 -> 0x1201 -> 0x1208 -> pick -> 0x1207 -> 0x914 ran, 0x914 failed locally
   with 0x800704DB four times (service 9 is not in the ticket) and XAM deleted the item (0x1206).
   Adding service 9 to the D7 list breaks the Live sign-in (80004005, the logon stops after
   GetRevocationList with no further request), so that is the next RE step. The reply
   needs exactly one entry with a 16-byte content key, a 284-byte RSA-2048 public key blob
   and up to 10 plain `http://` URLs. This service logs the request at
   `/xbos/FindMediaInstanceUrls.ashx` and answers 404;
2. downloads the URL. The body must be a signed, RC4-encrypted Microsoft Cabinet whose
   reserve is verified with that RSA key. The server chooses both keys, so OpenPak can produce
   valid files. A CAB writer is still to be built;
3. extracts a `LIVE` STFS package, verifies it with XeKeysVerifyRSASignature key 1 (Wharf: the
   OpenPak content signer via `--xsigner_public_key`; retail consoles: Microsoft's key, so
   modified kernels only), installs it through XContent, and deletes the item (0x1206).
   An STFS LIVE writer is still to be built.
