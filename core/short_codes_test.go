package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func TestIsShortCode(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"@a", true},
		{"@reproject", true},
		{"@map_1", true},
		{"@_x", true},
		{"@A", true},
		{"@A_B_2", true},
		{"@1bad", false},  // starts with digit
		{"@-x", false},    // hyphen not allowed
		{"@", false},      // bare @
		{"foo", false},    // no leading @
		{"@foo bar", false}, // space
		{"@foo.bar", false}, // dot
		{"019cf4bc-1111-7000-0000-000000000000", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isShortCode(tc.in); got != tc.want {
			t.Errorf("isShortCode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// parsePipelineDoc is a test helper that parses YAML to a top-level
// mapping node.
func parsePipelineDoc(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("parsing test pipeline: %v", err)
	}
	if len(doc.Content) == 0 {
		t.Fatal("empty document")
	}
	return doc.Content[0]
}

func TestResolveShortCodes_FreshPipeline(t *testing.T) {
	src := `name: p
version: "1"
blocks:
  - id: "@source"
    name: data.x
    inputs: []
    args: {}
  - id: "@reproject"
    name: raster.r
    inputs:
      - "@source"
    args: {}
  - id: "@clip"
    name: raster.c
    inputs:
      - block: "@reproject"
        output: tiles
    args: {}
`
	root := parsePipelineDoc(t, src)
	lock := Lockfile{Bindings: map[string]uuid.UUID{}}
	changed, err := ResolveShortCodes(root, &lock)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changed=true for a fresh pipeline")
	}
	want := []string{"@source", "@reproject", "@clip"}
	for _, code := range want {
		id, ok := lock.Bindings[code]
		if !ok {
			t.Errorf("missing binding for %s", code)
			continue
		}
		// Verify the value is a parseable UUID v7.
		if id.Version() != 7 {
			t.Errorf("binding %s: expected UUID v7, got v%d", code, id.Version())
		}
	}

	// Re-marshal and confirm none of the substituted scalars still
	// contain `@`.
	out, err := yaml.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	// Allow `@` in the comment-free YAML only inside `args`; here no
	// args use it, so it should be entirely gone from block ids and
	// inputs.
	if strings.Contains(string(out), "@") {
		t.Fatalf("substitution incomplete; output still contains '@':\n%s", out)
	}
}

func TestResolveShortCodes_ExplicitRefForm(t *testing.T) {
	src := `name: p
version: "1"
blocks:
  - id: "@a"
    name: x
    inputs: []
    args: {}
  - id: "@b"
    name: y
    inputs:
      - "@a"
      - block: "@a"
        output: special_output
    args: {}
`
	root := parsePipelineDoc(t, src)
	lock := Lockfile{Bindings: map[string]uuid.UUID{}}
	_, err := ResolveShortCodes(root, &lock)
	if err != nil {
		t.Fatal(err)
	}

	// The `output: special_output` scalar must NOT have been touched.
	out, _ := yaml.Marshal(root)
	if !strings.Contains(string(out), "special_output") {
		t.Fatalf("explicit `output` value lost:\n%s", out)
	}
	if strings.Count(string(out), lock.Bindings["@a"].String()) < 2 {
		t.Fatalf("expected @a's UUID to appear in both bare and explicit refs:\n%s", out)
	}
}

func TestResolveShortCodes_MixedFormat(t *testing.T) {
	concreteUUID := "019cf4bc-aaaa-7000-0000-000000000001"
	src := `name: p
version: "1"
blocks:
  - id: ` + concreteUUID + `
    name: a
    inputs: []
    args: {}
  - id: "@b"
    name: b
    inputs:
      - ` + concreteUUID + `
      - block: "@b"
        output: x
    args: {}
`
	root := parsePipelineDoc(t, src)
	lock := Lockfile{Bindings: map[string]uuid.UUID{}}
	_, err := ResolveShortCodes(root, &lock)
	if err != nil {
		t.Fatal(err)
	}

	out, _ := yaml.Marshal(root)
	// concrete UUID preserved verbatim
	if !strings.Contains(string(out), concreteUUID) {
		t.Fatalf("UUID-form id was modified:\n%s", out)
	}
	// short code @b resolved
	if _, ok := lock.Bindings["@b"]; !ok {
		t.Fatalf("@b binding not created")
	}
}

func TestResolveShortCodes_StableBindings(t *testing.T) {
	src := `name: p
version: "1"
blocks:
  - id: "@one"
    name: a
    inputs: []
    args: {}
  - id: "@two"
    name: b
    inputs:
      - "@one"
    args: {}
`
	rootA := parsePipelineDoc(t, src)
	lock := Lockfile{Bindings: map[string]uuid.UUID{}}
	if _, err := ResolveShortCodes(rootA, &lock); err != nil {
		t.Fatal(err)
	}
	originalOne := lock.Bindings["@one"]
	originalTwo := lock.Bindings["@two"]

	rootB := parsePipelineDoc(t, src)
	changed, err := ResolveShortCodes(rootB, &lock)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second resolution should not have added bindings")
	}
	if lock.Bindings["@one"] != originalOne || lock.Bindings["@two"] != originalTwo {
		t.Fatal("bindings changed across runs")
	}
}

func TestResolveShortCodes_AddNewCode(t *testing.T) {
	src1 := `name: p
version: "1"
blocks:
  - id: "@a"
    name: x
    inputs: []
    args: {}
`
	src2 := `name: p
version: "1"
blocks:
  - id: "@a"
    name: x
    inputs: []
    args: {}
  - id: "@b"
    name: y
    inputs:
      - "@a"
    args: {}
`
	lock := Lockfile{Bindings: map[string]uuid.UUID{}}
	if _, err := ResolveShortCodes(parsePipelineDoc(t, src1), &lock); err != nil {
		t.Fatal(err)
	}
	originalA := lock.Bindings["@a"]

	changed, err := ResolveShortCodes(parsePipelineDoc(t, src2), &lock)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("adding @b should set changed=true")
	}
	if lock.Bindings["@a"] != originalA {
		t.Fatal("@a binding mutated when @b was added")
	}
	if _, ok := lock.Bindings["@b"]; !ok {
		t.Fatal("@b not bound")
	}
}

func TestResolveShortCodes_RenameMintsFresh(t *testing.T) {
	src1 := `name: p
version: "1"
blocks:
  - id: "@reproject"
    name: x
    inputs: []
    args: {}
`
	src2 := `name: p
version: "1"
blocks:
  - id: "@reproject_v2"
    name: x
    inputs: []
    args: {}
`
	lock := Lockfile{Bindings: map[string]uuid.UUID{}}
	_, _ = ResolveShortCodes(parsePipelineDoc(t, src1), &lock)
	original := lock.Bindings["@reproject"]

	_, _ = ResolveShortCodes(parsePipelineDoc(t, src2), &lock)
	// Orphan binding preserved.
	if lock.Bindings["@reproject"] != original {
		t.Fatal("orphan binding @reproject mutated unexpectedly")
	}
	newID, ok := lock.Bindings["@reproject_v2"]
	if !ok {
		t.Fatal("@reproject_v2 not bound")
	}
	if newID == original {
		t.Fatal("renamed code reused old UUID")
	}
}

func TestResolveShortCodes_ArgsUntouched(t *testing.T) {
	src := `name: p
version: "1"
blocks:
  - id: "@a"
    name: x
    inputs: []
    args:
      tag: "@nope"
      nested:
        deeper: "@still_nope"
`
	root := parsePipelineDoc(t, src)
	lock := Lockfile{Bindings: map[string]uuid.UUID{}}
	if _, err := ResolveShortCodes(root, &lock); err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Bindings["@nope"]; ok {
		t.Fatal("@nope inside args was substituted")
	}
	if _, ok := lock.Bindings["@still_nope"]; ok {
		t.Fatal("@still_nope inside nested args was substituted")
	}
	if _, ok := lock.Bindings["@a"]; !ok {
		t.Fatal("@a (block id) should have been substituted")
	}

	out, _ := yaml.Marshal(root)
	if !strings.Contains(string(out), "@nope") {
		t.Fatalf("args value @nope was modified:\n%s", out)
	}
	if !strings.Contains(string(out), "@still_nope") {
		t.Fatalf("nested args value @still_nope was modified:\n%s", out)
	}
}

func TestResolveShortCodes_PipelineLevelIdRejected(t *testing.T) {
	src := `id: "@whatever"
name: p
version: "1"
blocks: []
`
	root := parsePipelineDoc(t, src)
	lock := Lockfile{Bindings: map[string]uuid.UUID{}}
	_, err := ResolveShortCodes(root, &lock)
	if err == nil {
		t.Fatal("expected error for top-level short-code id")
	}
	if !strings.Contains(err.Error(), "pipeline-level") {
		t.Fatalf("expected error mentioning pipeline-level, got: %v", err)
	}
}

func TestResolveShortCodes_DuplicateCodeBindsOnce(t *testing.T) {
	// Two blocks both declare id `@foo`.  The walker must NOT mint a new
	// UUID for the second occurrence -- it must reuse the first binding
	// so that the existing duplicate-id validation in ValidatePipeline
	// surfaces the authoring mistake.
	src := `name: p
version: "1"
blocks:
  - id: "@foo"
    name: a
    inputs: []
    args: {}
  - id: "@foo"
    name: b
    inputs: []
    args: {}
`
	root := parsePipelineDoc(t, src)
	lock := Lockfile{Bindings: map[string]uuid.UUID{}}
	if _, err := ResolveShortCodes(root, &lock); err != nil {
		t.Fatal(err)
	}
	if len(lock.Bindings) != 1 {
		t.Fatalf("expected exactly 1 binding for duplicate @foo, got %d", len(lock.Bindings))
	}
	// Confirm both block ids in the YAML now hold the same UUID.
	blocks := mappingValue(root, "blocks")
	var ids []string
	for _, b := range blocks.Content {
		if id := mappingValue(b, "id"); id != nil {
			ids = append(ids, id.Value)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d: %v", len(ids), ids)
	}
	if ids[0] != ids[1] {
		t.Fatalf("expected duplicate code to resolve to same UUID, got %s vs %s", ids[0], ids[1])
	}
}

func TestLoadAndResolvePipeline_FullCycle(t *testing.T) {
	dir := t.TempDir()
	pPath := filepath.Join(dir, "pipeline.yaml")
	os.WriteFile(pPath, []byte(`name: smoke
version: "1.0"
blocks:
  - id: "@source"
    name: data.x
    inputs: []
    args: {}
  - id: "@out"
    name: data.y
    inputs:
      - "@source"
    args: {}
`), 0644)

	pipeline, lock, wrote, err := LoadAndResolvePipeline(pPath)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("expected lockfile to be written on first run")
	}
	lockPath := LockfilePathFor(pPath)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lockfile not on disk: %v", err)
	}
	if len(lock.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(lock.Bindings))
	}

	// Verify the parsed Pipeline has UUID-typed Ids.
	if pipeline.Blocks[0].Id == uuid.Nil {
		t.Fatal("first block id is nil")
	}
	if pipeline.Blocks[0].Id != lock.Bindings["@source"] {
		t.Fatalf("source block id mismatch: %s vs %s", pipeline.Blocks[0].Id, lock.Bindings["@source"])
	}
	// Bare input reference resolves.
	if pipeline.Blocks[1].Inputs[0].ID != lock.Bindings["@source"] {
		t.Fatalf("input ref does not resolve to @source: %s", pipeline.Blocks[1].Inputs[0].ID)
	}
}

func TestLoadAndResolvePipeline_LockfileDeletedRegenerates(t *testing.T) {
	dir := t.TempDir()
	pPath := filepath.Join(dir, "pipeline.yaml")
	os.WriteFile(pPath, []byte(`name: smoke
version: "1.0"
blocks:
  - id: "@a"
    name: x
    inputs: []
    args: {}
`), 0644)

	_, lock1, _, err := LoadAndResolvePipeline(pPath)
	if err != nil {
		t.Fatal(err)
	}
	originalA := lock1.Bindings["@a"]

	if err := os.Remove(LockfilePathFor(pPath)); err != nil {
		t.Fatal(err)
	}

	_, lock2, wrote, err := LoadAndResolvePipeline(pPath)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("expected lockfile to be written after deletion")
	}
	if lock2.Bindings["@a"] == originalA {
		t.Fatal("deletion did not regenerate @a binding")
	}
}

func TestLoadAndResolvePipeline_PreservesUUIDOnlyPipelines(t *testing.T) {
	dir := t.TempDir()
	pPath := filepath.Join(dir, "pipeline.yaml")
	os.WriteFile(pPath, []byte(`id: 019cf4bc-0000-7000-0000-000000000000
name: smoke
version: "1.0"
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000001
    name: x
    inputs: []
    args: {}
`), 0644)

	_, _, wrote, err := LoadAndResolvePipeline(pPath)
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Fatal("UUID-only pipeline should not have created a lockfile")
	}
	if _, err := os.Stat(LockfilePathFor(pPath)); !os.IsNotExist(err) {
		t.Fatalf("lockfile should not exist for UUID-only pipeline: %v", err)
	}
}

func TestLoadAndResolvePipeline_InvalidLockfile(t *testing.T) {
	dir := t.TempDir()
	pPath := filepath.Join(dir, "pipeline.yaml")
	lockPath := LockfilePathFor(pPath)
	os.WriteFile(pPath, []byte(`name: smoke
version: "1.0"
blocks:
  - id: "@a"
    name: x
    inputs: []
    args: {}
`), 0644)
	os.WriteFile(lockPath, []byte(`bindings:
  "@a": not-a-uuid
`), 0644)

	_, _, _, err := LoadAndResolvePipeline(pPath)
	if err == nil {
		t.Fatal("expected error for invalid lockfile")
	}
	// The error should wrap ErrInvalidLockfile so the CLI can surface
	// the "delete the lockfile" hint.
	if !strings.Contains(err.Error(), "invalid lockfile") {
		t.Fatalf("expected invalid lockfile error, got: %v", err)
	}
}
