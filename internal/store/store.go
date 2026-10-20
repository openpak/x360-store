package store

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/openpak/x360-common/wire"
)

// ContentType is the XRL ("xon") wire type XAM sends and expects.
const ContentType = "xon/5"

// maxBody bounds XRL request bodies; the largest Lists request is well under 4 KiB.
const maxBody = 64 << 10

// Handler serves the public catalog (GET /v1/catalog) and the console's Lists
// (download queue) XRL requests, which the security gateway forwards with the
// signed-in users in wire.UsersHeader. Wrap it with wire.Guard.
type Handler struct {
	Catalog *Catalog
	Log     *slog.Logger
}

func (h *Handler) log() *slog.Logger {
	if h.Log != nil {
		return h.Log
	}
	return slog.Default()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.ToLower(r.URL.Path)
	switch {
	case path == "/healthz":
		w.WriteHeader(http.StatusOK)
	case path == "/v1/catalog":
		h.catalog(w, r)
	case strings.HasPrefix(path, "/lists/"):
		h.lists(w, r, path)
	default:
		h.log().Info("unhandled", "method", r.Method, "path", r.URL.Path)
		http.NotFound(w, r)
	}
}

// catalog lists the homebrew packages, optionally filtered by ?kind=game|app.
// An empty catalog is a valid answer: OpenPak publishes no package it cannot
// show to be redistributable (see README, "Packages").
func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	kind := r.URL.Query().Get("kind")
	items := []Item{}
	for _, it := range h.Catalog.Items {
		if kind == "" || it.Kind == kind {
			items = append(items, it)
		}
	}
	out := struct {
		Network string `json:"network"`
		Count   int    `json:"count"`
		Items   []Item `json:"items"`
	}{"xbox360", len(items), items}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	json.NewEncoder(w).Encode(out)
}

// Lists replies (wire layouts from the XAM 17559 XRL schema table, little-endian,
// no padding; see README):
//
//	0x1202 QueryDownloadQueues: u16 hr-ish attr, u16 queue count, queues...
//	0x1208 QueryDownloadItems:  u16 attr, u64 queue version, u16 item count, items...
//
// Nobody can queue anything yet, so both answer "no queue / no items".
var listsReplies = map[string][]byte{
	"/lists/querydownloadqueues.ashx": make([]byte, 4),
	"/lists/querydownloaditems.ashx":  make([]byte, 12),
}

func (h *Handler) lists(w http.ResponseWriter, r *http.Request, path string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return
	}
	reply, ok := listsReplies[path]
	h.log().Info("lists", "path", r.URL.Path, "users", r.Header.Get(wire.UsersHeader),
		"account", r.Header.Get(wire.AccountHeader), "body_len", len(body), "served", ok)
	if !ok || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(reply)))
	w.WriteHeader(http.StatusOK)
	w.Write(reply)
}
