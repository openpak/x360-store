package store

import (
	"bytes"
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
	h := &Handler{Catalog: &Catalog{}}
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
