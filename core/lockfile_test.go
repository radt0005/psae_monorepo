package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestLockfile_LoadMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.lock.yaml")
	lock, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("expected nil error for missing lockfile, got %v", err)
	}
	if lock.Bindings == nil {
		t.Fatal("expected non-nil Bindings map for missing lockfile")
	}
	if len(lock.Bindings) != 0 {
		t.Fatalf("expected empty Bindings map, got %d entries", len(lock.Bindings))
	}
}

func TestLockfile_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipeline.lock.yaml")
	original := Lockfile{
		Pipeline: "test-pipeline",
		Version:  "1.0",
		Bindings: map[string]uuid.UUID{
			"@source":    uuid.MustParse("019cf4bc-1111-7000-0000-000000000001"),
			"@reproject": uuid.MustParse("019cf4bc-2222-7000-0000-000000000002"),
			"@clip":      uuid.MustParse("019cf4bc-3333-7000-0000-000000000003"),
		},
	}
	if err := SaveLockfile(original, path); err != nil {
		t.Fatalf("SaveLockfile: %v", err)
	}
	loaded, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("LoadLockfile: %v", err)
	}
	if loaded.Pipeline != original.Pipeline || loaded.Version != original.Version {
		t.Fatalf("metadata mismatch: %+v vs %+v", loaded, original)
	}
	if len(loaded.Bindings) != len(original.Bindings) {
		t.Fatalf("bindings count mismatch: %d vs %d", len(loaded.Bindings), len(original.Bindings))
	}
	for code, id := range original.Bindings {
		if loaded.Bindings[code] != id {
			t.Errorf("binding %s: got %s, want %s", code, loaded.Bindings[code], id)
		}
	}
}

func TestLockfile_StableOrdering(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.lock.yaml")
	b := filepath.Join(tmp, "b.lock.yaml")

	bindings := map[string]uuid.UUID{
		"@charlie": uuid.MustParse("019cf4bc-3333-7000-0000-000000000003"),
		"@alpha":   uuid.MustParse("019cf4bc-1111-7000-0000-000000000001"),
		"@bravo":   uuid.MustParse("019cf4bc-2222-7000-0000-000000000002"),
	}
	lock := Lockfile{Pipeline: "p", Bindings: bindings}

	if err := SaveLockfile(lock, a); err != nil {
		t.Fatal(err)
	}
	if err := SaveLockfile(lock, b); err != nil {
		t.Fatal(err)
	}
	bytesA, _ := os.ReadFile(a)
	bytesB, _ := os.ReadFile(b)
	if !bytes.Equal(bytesA, bytesB) {
		t.Fatalf("expected identical bytes; got:\n--- a ---\n%s--- b ---\n%s", bytesA, bytesB)
	}

	// Order in the file should be lexicographic on the short codes.
	content := string(bytesA)
	posAlpha := bytes.Index(bytesA, []byte("@alpha"))
	posBravo := bytes.Index(bytesA, []byte("@bravo"))
	posCharlie := bytes.Index(bytesA, []byte("@charlie"))
	if posAlpha < 0 || posBravo < 0 || posCharlie < 0 {
		t.Fatalf("short codes missing from output:\n%s", content)
	}
	if !(posAlpha < posBravo && posBravo < posCharlie) {
		t.Fatalf("bindings not sorted; positions alpha=%d bravo=%d charlie=%d\ncontent:\n%s",
			posAlpha, posBravo, posCharlie, content)
	}
}

func TestLockfile_InvalidBindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.lock.yaml")
	os.WriteFile(path, []byte(`bindings:
  "@oops": not-a-uuid
`), 0644)

	_, err := LoadLockfile(path)
	if err == nil {
		t.Fatal("expected error for invalid UUID binding")
	}
	if !errors.Is(err, ErrInvalidLockfile) {
		t.Fatalf("expected ErrInvalidLockfile, got %v", err)
	}
}

func TestLockfile_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.lock.yaml")
	os.WriteFile(path, []byte("bindings: [this is not valid mapping"), 0644)

	_, err := LoadLockfile(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if !errors.Is(err, ErrInvalidLockfile) {
		t.Fatalf("expected ErrInvalidLockfile, got %v", err)
	}
}

func TestLockfilePathFor(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"pipeline.yaml", "pipeline.lock.yaml"},
		{"foo.yml", "foo.lock.yaml"},
		{"/abs/path/p.yaml", "/abs/path/p.lock.yaml"},
		{"workflows/foo.yaml", "workflows/foo.lock.yaml"},
		{"noext", "noext.lock.yaml"},
	}
	for _, tc := range cases {
		if got := LockfilePathFor(tc.in); got != tc.want {
			t.Errorf("LockfilePathFor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
