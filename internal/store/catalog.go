// Package store serves OpenPak's Xbox 360 homebrew catalog and the Live
// download-queue (Lists, Live service 10) requests XAM makes for it.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Item is one homebrew package. Every item names its licence and source so the
// catalog can only carry redistributable software.
type Item struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`     // "game" or "app"
	TitleID     string `json:"title_id"` // 8 hex digits, from the XEX execution info
	Package     string `json:"package"`  // STFS container magic: CON, LIVE or PIRS
	Size        int64  `json:"size"`
	SHA1        string `json:"sha1"` // of the whole package file
	URL         string `json:"url"`
	License     string `json:"license"` // SPDX identifier
	Source      string `json:"source"`  // public source repository
	Description string `json:"description,omitempty"`
}

// Catalog is the JSON document behind X360_STORE_CATALOG.
type Catalog struct {
	Items []Item `json:"items"`
}

var (
	hexTitle = regexp.MustCompile(`^[0-9A-Fa-f]{8}$`)
	hexSHA1  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	slug     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

// Validate rejects items that are incomplete or not demonstrably redistributable.
func (c *Catalog) Validate() error {
	seen := map[string]bool{}
	for i, it := range c.Items {
		bad := func(why string) error { return fmt.Errorf("item %d (%q): %s", i, it.ID, why) }
		switch {
		case !slug.MatchString(it.ID):
			return bad("id must be a lowercase slug")
		case seen[it.ID]:
			return bad("duplicate id")
		case strings.TrimSpace(it.Name) == "":
			return bad("name is required")
		case it.Kind != "game" && it.Kind != "app":
			return bad(`kind must be "game" or "app"`)
		case !hexTitle.MatchString(it.TitleID):
			return bad("title_id must be 8 hex digits")
		case it.Package != "CON" && it.Package != "LIVE" && it.Package != "PIRS":
			return bad("package must be CON, LIVE or PIRS")
		case it.Size <= 0:
			return bad("size is required")
		case !hexSHA1.MatchString(it.SHA1):
			return bad("sha1 must be 40 lowercase hex digits")
		case !strings.HasPrefix(it.URL, "http://") && !strings.HasPrefix(it.URL, "https://"):
			return bad("url must be http(s)")
		case strings.TrimSpace(it.License) == "":
			return bad("license (SPDX) is required")
		case !strings.HasPrefix(it.Source, "https://"):
			return bad("source must be a public https repository")
		}
		seen[it.ID] = true
	}
	return nil
}

// LoadCatalog reads and validates a catalog file. An empty path is the empty catalog.
func LoadCatalog(path string) (*Catalog, error) {
	c := &Catalog{Items: []Item{}}
	if path == "" {
		return c, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, err
	}
	if c.Items == nil {
		c.Items = []Item{}
	}
	return c, c.Validate()
}
