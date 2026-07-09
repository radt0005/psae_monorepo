package secretstore

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestSetGetListDelete(t *testing.T) {
	keyring.MockInit()

	if err := Set("db", "postgres://x"); err != nil {
		t.Fatalf("Set db: %v", err)
	}
	if err := Set("api", "key123"); err != nil {
		t.Fatalf("Set api: %v", err)
	}

	v, err := Get("db")
	if err != nil || v != "postgres://x" {
		t.Fatalf("Get db = %q, %v", v, err)
	}

	names, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 2 || names[0] != "api" || names[1] != "db" {
		t.Fatalf("List = %v, want sorted [api db]", names)
	}

	if err := Delete("db"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := Get("db"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	names, _ = List()
	if len(names) != 1 || names[0] != "api" {
		t.Fatalf("List after delete = %v, want [api]", names)
	}
}

func TestGetMissing(t *testing.T) {
	keyring.MockInit()
	if _, err := Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteMissingIsNoError(t *testing.T) {
	keyring.MockInit()
	if err := Delete("nope"); err != nil {
		t.Fatalf("Delete of missing secret should be a no-op, got %v", err)
	}
}

func TestSetIsIdempotentInIndex(t *testing.T) {
	keyring.MockInit()
	if err := Set("db", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := Set("db", "v2"); err != nil {
		t.Fatal(err)
	}
	names, _ := List()
	if len(names) != 1 {
		t.Fatalf("re-setting a name should not duplicate the index entry: %v", names)
	}
	v, _ := Get("db")
	if v != "v2" {
		t.Fatalf("re-set should overwrite value, got %q", v)
	}
}
