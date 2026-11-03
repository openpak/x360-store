package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/openpak/x360-common/pgstate"
)

func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("X360_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("X360_TEST_DATABASE_URL not set")
	}
	return dsn
}

func TestOpenQueuesRefusesWithoutDSN(t *testing.T) {
	if _, err := OpenQueues(context.Background(), "", Schema); err == nil {
		t.Fatal("started without a database URL")
	}
}

func TestQueueSurvivesRestart(t *testing.T) {
	dsn := testDSN(t)
	b := make([]byte, 6)
	rand.Read(b)
	schema := "t_store_" + hex.EncodeToString(b)
	ctx := context.Background()
	t.Cleanup(func() {
		if p, err := pgstate.Open(ctx, dsn, schema); err == nil {
			p.Pool().Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
			p.Close()
		}
	})
	q, err := OpenQueues(ctx, dsn, schema)
	if err != nil {
		t.Fatal(err)
	}
	const xuid = 0x0009000011112222
	id, err := q.Add(xuid, QueueItem{TitleID: 0x4F50FFFF, MediaType: 24, MediaID: "66acd000-77fe-1000-9115-d8024f50ffff",
		OfferID: "00000000-0000-4000-8000-00004f50ffff", OfferName: "Test", Size: 4096})
	if err != nil {
		t.Fatal(err)
	}
	q.Close()
	q, err = OpenQueues(ctx, dsn, schema) // a restart
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	u := q.m[xuid]
	if u == nil || len(u.Items) != 1 || u.Items[0].ID != id || u.Version != 2 {
		t.Fatalf("%+v", u)
	}
	if err := q.Clear(xuid); err != nil {
		t.Fatal(err)
	}
	q.Close()
	q, _ = OpenQueues(ctx, dsn, schema)
	defer q.Close()
	if q.m[xuid] != nil {
		t.Fatal("cleared queue came back")
	}
}
