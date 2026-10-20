package store

import (
	"crypto/subtle"
	"encoding/hex"
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
	Catalog  *Catalog
	Queues   *Queues
	AdminKey string // X360_STORE_ADMIN_KEY; empty disables the admin queue endpoint
	Log      *slog.Logger
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
	case path == "/v1/admin/queue":
		h.adminQueue(w, r)
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

func (h *Handler) lists(w http.ResponseWriter, r *http.Request, path string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return
	}
	var reply []byte
	ok := false
	if r.Method == http.MethodPost {
		reply, ok, err = h.Queues.Reply(path, body)
	}
	h.log().Info("lists", "path", r.URL.Path, "users", r.Header.Get(wire.UsersHeader),
		"account", r.Header.Get(wire.AccountHeader), "body_len", len(body), "served", ok, "reply_len", len(reply), "err", err)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(reply)))
	w.WriteHeader(http.StatusOK)
	w.Write(reply)
}

// adminQueue (POST, Bearer AdminKey) queues an item for one user; DELETE ?xuid= drops the
// user's queue. Internal network only: Traefik routes just /Lists to this service.
func (h *Handler) adminQueue(w http.ResponseWriter, r *http.Request) {
	if h.AdminKey == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+h.AdminKey)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		XUID      string `json:"xuid"`
		TitleID   string `json:"title_id"`
		MediaType uint32 `json:"media_type"`
		MediaID   string `json:"media_id"`
		OfferID   string `json:"offer_id"`
		OfferName string `json:"offer_name"`
		ContentID string `json:"content_id"`
		Size      uint64 `json:"size"`
	}
	if r.Method == http.MethodDelete {
		req.XUID = r.URL.Query().Get("xuid")
	} else if r.Method != http.MethodPost || json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req) != nil {
		http.Error(w, "POST a JSON item or DELETE ?xuid=", http.StatusBadRequest)
		return
	}
	xuid, err := strconv.ParseUint(req.XUID, 16, 64)
	if err != nil {
		http.Error(w, "xuid must be hex", http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodDelete {
		h.Queues.Clear(xuid)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	title, _ := strconv.ParseUint(req.TitleID, 16, 32)
	id, err := h.Queues.Add(xuid, QueueItem{TitleID: uint32(title), MediaType: req.MediaType, MediaID: req.MediaID,
		OfferID: req.OfferID, OfferName: req.OfferName, ContentID: req.ContentID, Size: req.Size})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.log().Info("queued", "xuid", req.XUID, "offer", req.OfferName, "item", hex.EncodeToString(id[:]))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"item_id": hex.EncodeToString(id[:])})
}
