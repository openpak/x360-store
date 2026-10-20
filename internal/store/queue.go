package store

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"regexp"
	"strconv"
	"sync"
)

// Download queue (Live "server-side download queue", Lists service 10). XAM
// itself polls it once per Live sign-in and then hourly, binds the queue to the
// console (0x1201), reads items (0x1208), downloads the first one addressed to
// it and deletes it (0x1206) or rewrites its state (0x1207). Layouts from the
// XAM 17559 schema table; behaviour from XAM's queue manager (README).
//
// State is in memory: a restart empties every queue, which XAM handles as
// "no queue".

// QueueItem is one queued download as XAM's item XML describes it.
type QueueItem struct {
	ID        [16]byte // queue item id, GUID wire order (u32, u16, u16 LE + 8 bytes)
	Flags     uint16   // opaque u16 XAM writes (1000) and expects echoed
	TitleID   uint32
	MediaType uint32 // marketplace media type (24 = DLC, 23 = arcade game, ...)
	MediaID   string // GUID string, no braces
	OfferID   string // GUID string, no braces
	OfferName string
	ContentID string // Base64 of the 20-byte STFS content id
	Size      uint64
	ConsoleID uint64 // 0 = any console the queue is bound to
	State     uint32
}

type queue struct {
	version uint64
	props   string // <RootNode><dest>..</dest><consoleRegionMask>..</consoleRegionMask></RootNode>
	items   []QueueItem
}

// Queues holds every user's queue, keyed by online XUID.
type Queues struct {
	mu sync.Mutex
	m  map[uint64]*queue
}

const queueID = 1
const itemFlags = 1000

var guidText = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Add queues an item for xuid (creating the queue) and returns the item id.
func (q *Queues) Add(xuid uint64, it QueueItem) ([16]byte, error) {
	if xuid == 0 || it.TitleID == 0 || it.MediaType == 0 || it.OfferName == "" ||
		!guidText.MatchString(it.MediaID) || !guidText.MatchString(it.OfferID) || it.Size == 0 {
		return [16]byte{}, errors.New("item needs xuid, title id, media type, media/offer GUIDs, offer name and size")
	}
	if _, err := rand.Read(it.ID[:]); err != nil {
		return [16]byte{}, err
	}
	it.Flags = itemFlags
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.m == nil {
		q.m = map[uint64]*queue{}
	}
	u := q.m[xuid]
	if u == nil {
		u = &queue{version: 1, props: "<RootNode><dest>0</dest><consoleRegionMask>0</consoleRegionMask></RootNode>"}
		q.m[xuid] = u
	}
	u.items = append(u.items, it)
	u.version++
	return it.ID, nil
}

// Clear drops xuid's queue.
func (q *Queues) Clear(xuid uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.m, xuid)
}

type reader struct {
	b   []byte
	err error
}

func (r *reader) take(n int) []byte {
	if r.err != nil || len(r.b) < n {
		r.err = errors.New("short request")
		return make([]byte, n)
	}
	v := r.b[:n]
	r.b = r.b[n:]
	return v
}
func (r *reader) u16() uint16 { return binary.LittleEndian.Uint16(r.take(2)) }
func (r *reader) u32() uint32 { return binary.LittleEndian.Uint32(r.take(4)) }
func (r *reader) u64() uint64 { return binary.LittleEndian.Uint64(r.take(8)) }

func le16(b []byte, v uint16) []byte { return binary.LittleEndian.AppendUint16(b, v) }
func le32(b []byte, v uint32) []byte { return binary.LittleEndian.AppendUint32(b, v) }
func le64(b []byte, v uint64) []byte { return binary.LittleEndian.AppendUint64(b, v) }

func esc(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func (it *QueueItem) xml() string {
	return "<RootNode><effectiveTitleId>" + strconv.FormatUint(uint64(it.TitleID), 10) +
		"</effectiveTitleId><mediaId>" + it.MediaID +
		"</mediaId><mediaType>" + strconv.FormatUint(uint64(it.MediaType), 10) +
		"</mediaType><ratingId>0</ratingId><consoleId>" + strconv.FormatUint(it.ConsoleID, 10) +
		"</consoleId><contentId>" + esc(it.ContentID) +
		"</contentId><offerId>" + it.OfferID +
		"</offerId><offerName>" + esc(it.OfferName) +
		"</offerName><deviceType>0</deviceType><size>" + strconv.FormatUint(it.Size, 10) +
		"</size><percent>0</percent><state>" + strconv.FormatUint(uint64(it.State), 10) +
		"</state><externalUrl></externalUrl><downloadSize>0</downloadSize></RootNode>"
}

// Reply builds the XRL reply for a Lists path, or ok=false for paths it does not serve.
func (q *Queues) Reply(path string, body []byte) (out []byte, ok bool, err error) {
	r := &reader{b: body}
	xuid := r.u64()
	q.mu.Lock()
	defer q.mu.Unlock()
	u := q.m[xuid]
	switch path {
	case "/lists/querydownloadqueues.ashx": // 0x1202
		if r.err != nil {
			return nil, true, r.err
		}
		if u == nil {
			return make([]byte, 4), true, nil // no queue
		}
		name := "DownloadQueue"
		out = le16(le16(nil, 0), 1)
		out = le32(le32(out, 0), queueID)
		out = le32(le64(out, u.version), uint32(len(u.items)))
		out = append(out, byte(len(name)))
		out = le16(out, uint16(len(u.props)))
		return append(append(out, name...), u.props...), true, nil
	case "/lists/querydownloaditems.ashx": // 0x1208
		r.u32() // queue id
		max, start := r.u32(), r.u32()
		r.u32()
		n := r.u16()
		want := map[[16]byte]bool{}
		for i := 0; i < int(n) && i < 100; i++ {
			want[[16]byte(r.take(16))] = true
		}
		if r.err != nil {
			return nil, true, r.err
		}
		if u == nil {
			return make([]byte, 12), true, nil
		}
		var sel []QueueItem
		for _, it := range u.items {
			if len(want) == 0 || want[it.ID] {
				sel = append(sel, it)
			}
		}
		total := len(sel)
		if int(start) < len(sel) {
			sel = sel[start:]
		} else {
			sel = nil
		}
		if max > 0 && int(max) < len(sel) {
			sel = sel[:max]
		}
		out = le16(le64(le16(nil, uint16(total)), u.version), uint16(len(sel)))
		for _, it := range sel {
			x := it.xml()
			out = append(out, it.ID[:]...)
			out = le16(le16(out, it.Flags), uint16(len(x)))
			out = append(out, x...)
		}
		return out, true, nil
	case "/lists/modifydownloadqueue.ashx": // 0x1201: bind the queue to the console
		r.u32()
		r.u64()
		props := string(r.take(int(r.u16())))
		if r.err != nil {
			return nil, true, r.err
		}
		if u == nil {
			return nil, true, errors.New("no queue")
		}
		u.props = props
		u.version++
		return le64(nil, u.version), true, nil
	case "/lists/deletedownloaditems.ashx": // 0x1206
		r.u32()
		r.u64()
		n := r.u16()
		del := map[[16]byte]bool{}
		for i := 0; i < int(n) && i < 100; i++ {
			del[[16]byte(r.take(16))] = true
		}
		if r.err != nil {
			return nil, true, r.err
		}
		if u == nil {
			return nil, true, errors.New("no queue")
		}
		keep := u.items[:0]
		for _, it := range u.items {
			if !del[it.ID] {
				keep = append(keep, it)
			}
		}
		u.items = keep
		u.version++
		return le64(nil, u.version), true, nil
	case "/lists/modifydownloaditems.ashx": // 0x1207: XAM rewrites state/percent
		r.u32()
		r.u64()
		n := r.u16()
		for i := 0; i < int(n) && i < 100 && r.err == nil; i++ {
			id := [16]byte(r.take(16))
			r.u16()
			x := r.take(int(r.u16()))
			if u == nil || r.err != nil {
				continue
			}
			for j := range u.items {
				if u.items[j].ID == id {
					var st struct {
						State uint32 `xml:"state"`
					}
					if xml.Unmarshal(x, &st) == nil {
						u.items[j].State = st.State
					}
				}
			}
		}
		if r.err != nil {
			return nil, true, r.err
		}
		if u == nil {
			return nil, true, errors.New("no queue")
		}
		u.version++
		return le64(nil, u.version), true, nil
	}
	return nil, false, nil
}
