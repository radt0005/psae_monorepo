package store

import (
	"context"
	"errors"
	"testing"
)

func newTestStore(t *testing.T) *GormStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	return s
}

func TestUpsertGetListDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	sec := Secret{
		OwnerID: "alice", Name: "db",
		Ciphertext: []byte("ct"), ValueNonce: []byte("n1"),
		WrappedDEK: []byte("wd"), DEKNonce: []byte("n2"), KEKID: "v1",
	}
	if err := s.Upsert(ctx, sec); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Get(ctx, "alice", "db")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got.Ciphertext) != "ct" || got.KEKID != "v1" {
		t.Fatalf("Get = %+v", got)
	}

	// Upsert overwrites the value for the same (owner, name).
	sec.Ciphertext = []byte("ct2")
	if err := s.Upsert(ctx, sec); err != nil {
		t.Fatalf("re-Upsert: %v", err)
	}
	got, _ = s.Get(ctx, "alice", "db")
	if string(got.Ciphertext) != "ct2" {
		t.Fatalf("update failed: %q", got.Ciphertext)
	}

	// Secrets are isolated by owner.
	if err := s.Upsert(ctx, Secret{OwnerID: "bob", Name: "db", KEKID: "v1", Ciphertext: []byte("x")}); err != nil {
		t.Fatal(err)
	}
	names, _ := s.ListNames(ctx, "alice")
	if len(names) != 1 || names[0] != "db" {
		t.Fatalf("alice names = %v", names)
	}
	if _, err := s.Get(ctx, "alice", "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	if err := s.Delete(ctx, "alice", "db"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "alice", "db"); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected secret to be deleted")
	}
	if err := s.Delete(ctx, "alice", "db"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting a missing secret should return ErrNotFound, got %v", err)
	}
	// bob's secret is untouched.
	if _, err := s.Get(ctx, "bob", "db"); err != nil {
		t.Fatalf("bob's secret should survive alice's delete: %v", err)
	}
}

func TestAudit(t *testing.T) {
	s := newTestStore(t)
	if err := s.Audit(context.Background(), AuditRecord{
		OwnerID: "alice", Action: "set", SecretNames: "db",
	}); err != nil {
		t.Fatalf("Audit: %v", err)
	}
}
