package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// nestedFixture builds a fake pipeline directory tree for a depth-2 run:
//
//	M1 (map, depth 0)   → expansion: 2 outer items
//	M2 (map, depth 1)   → instances M2.0 (3 items), M2.1 (1 item)
//	X  (standard, 2)    → work dirs X.<i>.<j> with one output file each
//	R2 (reduce, 1)      → gathers X.<i>.*
//	R1 (reduce, 0)      → gathers R2.*
//	model (standard, 0) → broadcast into X
//	ref (standard, 1)   → per-outer-instance broadcast into X (dirs ref.<i>)
type nestedFixture struct {
	pipelineDir string
	ids         map[string]uuid.UUID
	depths      map[uuid.UUID]int
	manifests   map[uuid.UUID]BlockManifest // keyed by block UUID for SetupInputSymlinks
	innerCounts []int                       // items per outer index
}

func writeExpansion(t *testing.T, workDir, outputName string, items []ExpansionItem) {
	t.Helper()
	dir := filepath.Join(workDir, "outputs", outputName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(ExpansionManifest{Items: items})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "expansion.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func writeOutput(t *testing.T, workDir, outputName, fileName, content string) {
	t.Helper()
	dir := filepath.Join(workDir, "outputs", outputName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func buildNestedFixture(t *testing.T) *nestedFixture {
	t.Helper()
	f := &nestedFixture{
		pipelineDir: t.TempDir(),
		ids:         map[string]uuid.UUID{},
		depths:      map[uuid.UUID]int{},
		manifests:   map[uuid.UUID]BlockManifest{},
		innerCounts: []int{3, 1},
	}
	depths := map[string]int{
		"M1": 0, "M2": 1, "X": 2, "R2": 1, "R1": 0, "model": 0, "ref": 1,
	}
	kinds := map[string]BlockKind{
		"M1": BlockKindMap, "M2": BlockKindMap,
		"X": BlockKindStandard, "model": BlockKindStandard, "ref": BlockKindStandard,
		"R2": BlockKindReduce, "R1": BlockKindReduce,
	}
	for name, d := range depths {
		id := uuid.New()
		f.ids[name] = id
		f.depths[id] = d
		f.manifests[id] = BlockManifest{ID: name, Kind: kinds[name]}
	}

	dir := func(name string, indices ...int) string {
		return filepath.Join(f.pipelineDir, FormatInvocationID(f.ids[name], indices))
	}

	// M1: one instance, 2 outer items (files live in M1's work dir).
	m1Dir := dir("M1")
	var outerItems []ExpansionItem
	for i := 0; i < 2; i++ {
		rel := filepath.Join("inputs", "source", fmt.Sprintf("outer_%d.dat", i))
		abs := filepath.Join(m1Dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(fmt.Sprintf("outer %d", i)), 0644); err != nil {
			t.Fatal(err)
		}
		outerItems = append(outerItems, ExpansionItem{Path: rel, Key: fmt.Sprintf("outer_%d", i)})
	}
	writeExpansion(t, m1Dir, "manifest", outerItems)

	// M2 instances: M2.0 (3 items), M2.1 (1 item).
	for i, n := range f.innerCounts {
		m2Dir := dir("M2", i)
		var items []ExpansionItem
		for j := 0; j < n; j++ {
			rel := filepath.Join("inputs", "source", fmt.Sprintf("inner_%d_%d.dat", i, j))
			abs := filepath.Join(m2Dir, rel)
			if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(abs, []byte(fmt.Sprintf("inner %d.%d", i, j)), 0644); err != nil {
				t.Fatal(err)
			}
			items = append(items, ExpansionItem{Path: rel, Key: fmt.Sprintf("inner_%d_%d", i, j)})
		}
		writeExpansion(t, m2Dir, "manifest", items)
	}

	// X work dirs with outputs.
	for i, n := range f.innerCounts {
		for j := 0; j < n; j++ {
			writeOutput(t, dir("X", i, j), "result", "out.dat", fmt.Sprintf("X %d.%d", i, j))
		}
	}

	// R2 instance outputs.
	for i := range f.innerCounts {
		writeOutput(t, dir("R2", i), "merged", "merged.dat", fmt.Sprintf("R2 %d", i))
	}

	// Broadcast sources.
	writeOutput(t, dir("model"), "weights", "model.bin", "weights")
	for i := range f.innerCounts {
		writeOutput(t, dir("ref", i), "reference", "ref.dat", fmt.Sprintf("ref %d", i))
	}

	return f
}

func (f *nestedFixture) invocation(name string, indices ...int) BlockInvocation {
	return BlockInvocation{Id: f.ids[name], BlockId: name, MapIndices: indices}
}

func readLink(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading link target %s: %v", target, err)
	}
	return string(data)
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// TestNestedSymlinksInnerWorkBlock: X.1.0 consumes item 0 of M2's
// instance 1 expansion, plus a depth-0 and a depth-1 broadcast.
func TestNestedSymlinksInnerWorkBlock(t *testing.T) {
	f := buildNestedFixture(t)
	workDir := t.TempDir()

	inv := f.invocation("X", 1, 0)
	resolved := map[string]ResolvedInput{
		"tile":      {InputName: "tile", SourceBlockID: f.ids["M2"], SourceOutputName: "manifest"},
		"reference": {InputName: "reference", SourceBlockID: f.ids["ref"], SourceOutputName: "reference"},
		"weights":   {InputName: "weights", SourceBlockID: f.ids["model"], SourceOutputName: "weights"},
	}
	err := SetupInputSymlinks(workDir, resolved, f.pipelineDir, inv,
		f.manifests[f.ids["X"]], f.manifests, f.depths)
	if err != nil {
		t.Fatalf("SetupInputSymlinks: %v", err)
	}

	if got := readLink(t, filepath.Join(workDir, "inputs", "tile", "inner_1_0.dat")); got != "inner 1.0" {
		t.Errorf("tile content = %q, want %q", got, "inner 1.0")
	}
	if got := readLink(t, filepath.Join(workDir, "inputs", "reference", "ref.dat")); got != "ref 1" {
		t.Errorf("reference content = %q, want %q (instance-1 broadcast)", got, "ref 1")
	}
	if got := readLink(t, filepath.Join(workDir, "inputs", "weights", "model.bin")); got != "weights" {
		t.Errorf("weights content = %q, want %q", got, "weights")
	}
}

// TestNestedSymlinksInnerReduce: R2.0 gathers exactly X.0.* (3 siblings),
// not X.1.*.
func TestNestedSymlinksInnerReduce(t *testing.T) {
	f := buildNestedFixture(t)
	workDir := t.TempDir()

	inv := f.invocation("R2", 0)
	resolved := map[string]ResolvedInput{
		"parts": {InputName: "parts", SourceBlockID: f.ids["X"], SourceOutputName: "result"},
	}
	err := SetupInputSymlinks(workDir, resolved, f.pipelineDir, inv,
		f.manifests[f.ids["R2"]], f.manifests, f.depths)
	if err != nil {
		t.Fatalf("SetupInputSymlinks: %v", err)
	}

	names := listDir(t, filepath.Join(workDir, "inputs", "parts"))
	want := []string{"0_out.dat", "1_out.dat", "2_out.dat"}
	if len(names) != len(want) {
		t.Fatalf("gathered %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("gathered %v, want %v", names, want)
		}
	}
	for j := 0; j < 3; j++ {
		got := readLink(t, filepath.Join(workDir, "inputs", "parts", fmt.Sprintf("%d_out.dat", j)))
		if got != fmt.Sprintf("X 0.%d", j) {
			t.Errorf("parts[%d] = %q, want %q", j, got, fmt.Sprintf("X 0.%d", j))
		}
	}
}

// TestNestedSymlinksOuterReduce: R1 gathers exactly R2.* (one per outer
// instance), not the deeper X dirs.
func TestNestedSymlinksOuterReduce(t *testing.T) {
	f := buildNestedFixture(t)
	workDir := t.TempDir()

	inv := f.invocation("R1")
	resolved := map[string]ResolvedInput{
		"parts": {InputName: "parts", SourceBlockID: f.ids["R2"], SourceOutputName: "merged"},
	}
	err := SetupInputSymlinks(workDir, resolved, f.pipelineDir, inv,
		f.manifests[f.ids["R1"]], f.manifests, f.depths)
	if err != nil {
		t.Fatalf("SetupInputSymlinks: %v", err)
	}

	names := listDir(t, filepath.Join(workDir, "inputs", "parts"))
	want := []string{"0_merged.dat", "1_merged.dat"}
	if len(names) != len(want) {
		t.Fatalf("gathered %v, want %v", names, want)
	}
	for i := range f.innerCounts {
		got := readLink(t, filepath.Join(workDir, "inputs", "parts", fmt.Sprintf("%d_merged.dat", i)))
		if got != fmt.Sprintf("R2 %d", i) {
			t.Errorf("parts[%d] = %q, want %q", i, got, fmt.Sprintf("R2 %d", i))
		}
	}
}

// TestNestedSymlinksInnerMapConsumesOuterItem: M2.1 (the inner map block,
// itself a mapped invocation of M1's context) consumes outer item 1 from
// M1's expansion.
func TestNestedSymlinksInnerMapConsumesOuterItem(t *testing.T) {
	f := buildNestedFixture(t)
	workDir := t.TempDir()

	inv := f.invocation("M2", 1)
	resolved := map[string]ResolvedInput{
		"source": {InputName: "source", SourceBlockID: f.ids["M1"], SourceOutputName: "manifest"},
	}
	err := SetupInputSymlinks(workDir, resolved, f.pipelineDir, inv,
		f.manifests[f.ids["M2"]], f.manifests, f.depths)
	if err != nil {
		t.Fatalf("SetupInputSymlinks: %v", err)
	}

	if got := readLink(t, filepath.Join(workDir, "inputs", "source", "outer_1.dat")); got != "outer 1" {
		t.Errorf("source content = %q, want %q", got, "outer 1")
	}
}

// TestReduceSiblingNumericOrder: with more than 9 siblings the gather
// order must be numeric, not lexicographic (regression: ".10" used to
// sort before ".2").
func TestReduceSiblingNumericOrder(t *testing.T) {
	pipelineDir := t.TempDir()
	workDir := t.TempDir()
	xID := uuid.New()
	rID := uuid.New()

	const n = 12
	for j := 0; j < n; j++ {
		d := filepath.Join(pipelineDir, fmt.Sprintf("%s.%d", xID, j))
		if err := os.MkdirAll(filepath.Join(d, "outputs", "result"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "outputs", "result", "part.dat"), []byte(fmt.Sprintf("%d", j)), 0644); err != nil {
			t.Fatal(err)
		}
	}

	depths := map[uuid.UUID]int{xID: 1, rID: 0}
	manifests := map[uuid.UUID]BlockManifest{
		xID: {ID: "x", Kind: BlockKindStandard},
		rID: {ID: "r", Kind: BlockKindReduce},
	}
	inv := BlockInvocation{Id: rID, BlockId: "r"}
	resolved := map[string]ResolvedInput{
		"parts": {InputName: "parts", SourceBlockID: xID, SourceOutputName: "result"},
	}
	if err := SetupInputSymlinks(workDir, resolved, pipelineDir, inv, manifests[rID], manifests, depths); err != nil {
		t.Fatalf("SetupInputSymlinks: %v", err)
	}

	// Every sibling must be present exactly once with its numeric tag.
	for j := 0; j < n; j++ {
		link := filepath.Join(workDir, "inputs", "parts", fmt.Sprintf("%d_part.dat", j))
		got := readLink(t, link)
		if got != fmt.Sprintf("%d", j) {
			t.Errorf("parts[%d] = %q, want %q", j, got, fmt.Sprintf("%d", j))
		}
	}

	// And gatherSiblingDirs itself must return them in numeric order.
	sibs := gatherSiblingDirs(pipelineDir, xID, nil)
	if len(sibs) != n {
		t.Fatalf("gathered %d siblings, want %d", len(sibs), n)
	}
	for j, s := range sibs {
		if s.item != j {
			t.Fatalf("sibling order: position %d has item %d (numeric sort broken)", j, s.item)
		}
	}
}
