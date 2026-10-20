package store

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmptyCatalog(t *testing.T) {
	c, err := LoadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	(&Handler{Catalog: c}).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/catalog?kind=game", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"items":[]`) || !strings.Contains(rec.Body.String(), `"count":0`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestCatalogValidation(t *testing.T) {
	good := Item{ID: "demo-app", Name: "Demo", Kind: "app", TitleID: "4F50AA01", Package: "CON", Size: 1,
		SHA1: strings.Repeat("a", 40), URL: "https://example.org/demo", License: "MIT", Source: "https://example.org/src"}
	for name, mut := range map[string]func(*Item){
		"no license": func(i *Item) { i.License = "" },
		"no source":  func(i *Item) { i.Source = "" },
		"bad title":  func(i *Item) { i.TitleID = "xyz" },
		"bad kind":   func(i *Item) { i.Kind = "dlc" },
		"bad pkg":    func(i *Item) { i.Package = "XEX" },
	} {
		it := good
		mut(&it)
		if (&Catalog{Items: []Item{it}}).Validate() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := (&Catalog{Items: []Item{good}}).Validate(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "c.json")
	b, _ := json.Marshal(Catalog{Items: []Item{good}})
	os.WriteFile(p, b, 0o600)
	c, err := LoadCatalog(p)
	if err != nil || len(c.Items) != 1 {
		t.Fatal(c, err)
	}
	rec := httptest.NewRecorder()
	(&Handler{Catalog: c}).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/catalog?kind=game", nil))
	if !strings.Contains(rec.Body.String(), `"count":0`) {
		t.Fatalf("kind filter: %s", rec.Body)
	}
}

func TestListsEmptyQueue(t *testing.T) {
	h := &Handler{Catalog: &Catalog{}, Queues: &Queues{}}
	for path, n := range map[string]int{"/Lists/QueryDownloadQueues.ashx": 4, "/Lists/QueryDownloadItems.ashx": 12} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("POST", path, bytes.NewReader(make([]byte, 32))))
		if rec.Code != 200 || rec.Body.Len() != n || rec.Header().Get("Content-Type") != ContentType ||
			!bytes.Equal(rec.Body.Bytes(), make([]byte, n)) {
			t.Errorf("%s: %d %x", path, rec.Code, rec.Body.Bytes())
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/Lists/InsertItems.ashx", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown Lists path: %d", rec.Code)
	}
}

func TestQueueFlow(t *testing.T) {
	q := &Queues{}
	const xuid = 0x0009000011112222
	id, err := q.Add(xuid, QueueItem{TitleID: 0x4F50FFFF, MediaType: 24, MediaID: "66acd000-77fe-1000-9115-d8024f50ffff",
		OfferID: "00000000-0000-4000-8000-00004f50ffff", OfferName: "Test", Size: 4096})
	if err != nil {
		t.Fatal(err)
	}
	req := func(fields ...any) []byte {
		var b bytes.Buffer
		binary.Write(&b, binary.LittleEndian, uint64(xuid))
		for _, f := range fields {
			binary.Write(&b, binary.LittleEndian, f)
		}
		return b.Bytes()
	}
	out, ok, err := q.Reply("/lists/querydownloadqueues.ashx", req(uint32(0x80000000), uint64(0), uint32(1), uint32(0), uint32(1)))
	if !ok || err != nil || binary.LittleEndian.Uint16(out[2:]) != 1 || binary.LittleEndian.Uint32(out[8:]) != 1 || binary.LittleEndian.Uint64(out[12:]) != 2 ||
		binary.LittleEndian.Uint32(out[20:]) != 1 || !bytes.Contains(out, []byte("DownloadQueue<RootNode><dest>0</dest>")) {
		t.Fatalf("0x1202: %v %x", err, out)
	}
	props := "<RootNode><dest>1234</dest><consoleRegionMask>255</consoleRegionMask></RootNode>"
	out, _, err = q.Reply("/lists/modifydownloadqueue.ashx", append(req(uint32(1), uint64(2), uint16(len(props))), props...))
	if err != nil || binary.LittleEndian.Uint64(out) != 3 {
		t.Fatalf("0x1201: %v %x", err, out)
	}
	out, _, err = q.Reply("/lists/querydownloaditems.ashx", req(uint32(1), uint32(2), uint32(0), uint32(1), uint16(0)))
	if err != nil || binary.LittleEndian.Uint16(out) != 1 || binary.LittleEndian.Uint64(out[2:]) != 3 ||
		binary.LittleEndian.Uint16(out[10:]) != 1 || !bytes.Equal(out[12:28], id[:]) ||
		binary.LittleEndian.Uint16(out[28:]) != 1000 || !bytes.Contains(out, []byte("<offerName>Test</offerName>")) {
		t.Fatalf("0x1208: %v %q", err, out)
	}
	out, _, err = q.Reply("/lists/deletedownloaditems.ashx", append(req(uint32(1), uint64(3), uint16(1)), id[:]...))
	if err != nil || binary.LittleEndian.Uint64(out) != 4 {
		t.Fatalf("0x1206: %v %x", err, out)
	}
	out, _, _ = q.Reply("/lists/querydownloaditems.ashx", req(uint32(1), uint32(2), uint32(0), uint32(1), uint16(0)))
	if binary.LittleEndian.Uint16(out) != 0 || binary.LittleEndian.Uint16(out[10:]) != 0 {
		t.Fatalf("after delete: %x", out)
	}
	if _, _, err := q.Reply("/lists/querydownloadqueues.ashx", []byte{1, 2}); err == nil {
		t.Fatal("short request accepted")
	}
}
