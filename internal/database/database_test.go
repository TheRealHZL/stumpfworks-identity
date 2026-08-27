package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestOpenUpgradesExistingClientTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE clients (id INTEGER PRIMARY KEY AUTOINCREMENT, client_id TEXT NOT NULL UNIQUE, token_hash TEXT NOT NULL, version TEXT NOT NULL DEFAULT '', network_status TEXT NOT NULL DEFAULT 'unknown', ad_status TEXT NOT NULL DEFAULT 'unknown', camera_status TEXT NOT NULL DEFAULT 'unknown', kerberos_status TEXT NOT NULL DEFAULT 'unknown', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, last_seen_at DATETIME); INSERT INTO clients(client_id,token_hash) VALUES('legacy','hash')`)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	c, err := store.ClientByID(context.Background(), "legacy")
	if err != nil || !c.Enabled {
		t.Fatalf("legacy client was not enabled after migration: %+v, %v", c, err)
	}
}

func TestClientUpdateMetadata(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateClient(t.Context(), "client01", "hash"); err != nil {
		t.Fatal(err)
	}
	updatedAt := time.Now().UTC().Truncate(time.Second)
	update := &ClientUpdate{Version: "1.2.0", Status: "success", UpdatedAt: updatedAt, RollbackAvailable: true}
	if err = store.UpdateClientStatusWithUpdate(t.Context(), "client01", "1.2.0", "ok", "ok", "ok", "ok", update); err != nil {
		t.Fatal(err)
	}
	client, err := store.ClientByID(t.Context(), "client01")
	if err != nil || client.LastUpdateVersion != "1.2.0" || client.LastUpdateStatus != "success" || client.LastUpdateAt == nil || !client.RollbackAvailable {
		t.Fatalf("unexpected update metadata: %+v, %v", client, err)
	}
}

func TestRotateAndDisableClient(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateClient(t.Context(), "client01", "old"); err != nil {
		t.Fatal(err)
	}
	if err = store.RotateClientToken(t.Context(), "client01", "new"); err != nil {
		t.Fatal(err)
	}
	if err = store.SetClientEnabled(t.Context(), "client01", false); err != nil {
		t.Fatal(err)
	}
	c, err := store.ClientByID(t.Context(), "client01")
	if err != nil || c.TokenHash != "new" || c.Enabled {
		t.Fatalf("unexpected client state: %+v, %v", c, err)
	}
}
