package core

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// contextTreeFixture builds a pipeline + manifests from a compact block
// spec: name → kind and inputs.  Block UUIDs are minted per name.
type ctBlock struct {
	name   string
	kind   BlockKind
	inputs []string
}

func buildContextTreeFixture(t *testing.T, blocks []ctBlock) (Pipeline, map[string]BlockManifest, DependencyGraph, map[string]uuid.UUID) {
	t.Helper()
	ids := make(map[string]uuid.UUID, len(blocks))
	for _, b := range blocks {
		ids[b.name] = uuid.New()
	}
	p := Pipeline{Id: uuid.New(), Name: "fixture", Version: "1.0"}
	manifests := make(map[string]BlockManifest, len(blocks))
	for _, b := range blocks {
		var refs []InputRef
		for _, in := range b.inputs {
			id, ok := ids[in]
			if !ok {
				t.Fatalf("fixture references unknown block %q", in)
			}
			refs = append(refs, InputRef{ID: id})
		}
		if refs == nil {
			refs = []InputRef{}
		}
		p.Blocks = append(p.Blocks, PipelineBlock{
			Id: ids[b.name], Name: b.name, Inputs: refs, Args: map[string]any{},
		})
		m := BlockManifest{ID: b.name, Kind: b.kind}
		switch b.kind {
		case BlockKindMap:
			m.Inputs = map[string]InputDeclaration{"source": {Type: "collection"}}
			m.Outputs = map[string]OutputDeclaration{"manifest": {Type: "expansion"}}
		case BlockKindReduce:
			m.Inputs = map[string]InputDeclaration{"parts": {Type: "collection"}}
			m.Outputs = map[string]OutputDeclaration{"result": {Type: "file"}}
		default:
			m.Inputs = map[string]InputDeclaration{"data": {Type: "file"}}
			m.Outputs = map[string]OutputDeclaration{"result": {Type: "file"}}
		}
		manifests[b.name] = m
	}
	g, err := BuildDependencyGraph(p)
	if err != nil {
		t.Fatalf("building dependency graph: %v", err)
	}
	return p, manifests, g, ids
}

func pathOf(t *testing.T, tree *ContextTree, ids map[string]uuid.UUID, name string, wantDepth int, wantChain ...string) {
	t.Helper()
	path := tree.Paths[ids[name]]
	if len(path) != wantDepth {
		t.Fatalf("%s: depth = %d, want %d (path %v)", name, len(path), wantDepth, path)
	}
	for i, chainName := range wantChain {
		if path[i] != ids[chainName] {
			t.Errorf("%s: path[%d] = %s, want %s (%s)", name, i, path[i], ids[chainName], chainName)
		}
	}
}

func TestBuildContextTreeDepth2Chain(t *testing.T) {
	// src → M1 → M2 → X → R2 → R1 → final
	p, manifests, g, ids := buildContextTreeFixture(t, []ctBlock{
		{"src", BlockKindStandard, nil},
		{"M1", BlockKindMap, []string{"src"}},
		{"M2", BlockKindMap, []string{"M1"}},
		{"X", BlockKindStandard, []string{"M2"}},
		{"R2", BlockKindReduce, []string{"X"}},
		{"R1", BlockKindReduce, []string{"R2"}},
		{"final", BlockKindStandard, []string{"R1"}},
	})

	tree, err := BuildContextTree(p, manifests, g)
	if err != nil {
		t.Fatalf("BuildContextTree: %v", err)
	}

	pathOf(t, tree, ids, "src", 0)
	pathOf(t, tree, ids, "M1", 0)
	pathOf(t, tree, ids, "M2", 1, "M1")
	pathOf(t, tree, ids, "X", 2, "M1", "M2")
	pathOf(t, tree, ids, "R2", 1, "M1")
	pathOf(t, tree, ids, "R1", 0)
	pathOf(t, tree, ids, "final", 0)

	// Membership: M1's context holds M2 and R2; M2's context holds X.
	if !tree.Members[ids["M1"]][ids["M2"]] || !tree.Members[ids["M1"]][ids["R2"]] {
		t.Errorf("M1 members = %v, want M2 and R2", tree.Members[ids["M1"]])
	}
	if len(tree.Members[ids["M1"]]) != 2 {
		t.Errorf("M1 should have exactly 2 members, got %v", tree.Members[ids["M1"]])
	}
	if !tree.Members[ids["M2"]][ids["X"]] || len(tree.Members[ids["M2"]]) != 1 {
		t.Errorf("M2 members = %v, want exactly X", tree.Members[ids["M2"]])
	}

	// Reduces: R2 closes M2, R1 closes M1.
	if got := tree.Reduces[ids["M2"]]; len(got) != 1 || got[0] != ids["R2"] {
		t.Errorf("M2 reduces = %v, want [R2]", got)
	}
	if got := tree.Reduces[ids["M1"]]; len(got) != 1 || got[0] != ids["R1"] {
		t.Errorf("M1 reduces = %v, want [R1]", got)
	}
}

func TestBuildContextTreeSiblingInnerContexts(t *testing.T) {
	// One outer context with two parallel inner map/reduce pairs:
	// M1 → (M2a → Xa → R2a) and (M2b → Xb → R2b) → join → R1
	p, manifests, g, ids := buildContextTreeFixture(t, []ctBlock{
		{"M1", BlockKindMap, nil},
		{"M2a", BlockKindMap, []string{"M1"}},
		{"Xa", BlockKindStandard, []string{"M2a"}},
		{"R2a", BlockKindReduce, []string{"Xa"}},
		{"M2b", BlockKindMap, []string{"M1"}},
		{"Xb", BlockKindStandard, []string{"M2b"}},
		{"R2b", BlockKindReduce, []string{"Xb"}},
		{"join", BlockKindStandard, []string{"R2a", "R2b"}},
		{"R1", BlockKindReduce, []string{"join"}},
	})

	tree, err := BuildContextTree(p, manifests, g)
	if err != nil {
		t.Fatalf("BuildContextTree: %v", err)
	}

	pathOf(t, tree, ids, "Xa", 2, "M1", "M2a")
	pathOf(t, tree, ids, "Xb", 2, "M1", "M2b")
	pathOf(t, tree, ids, "R2a", 1, "M1")
	pathOf(t, tree, ids, "R2b", 1, "M1")
	pathOf(t, tree, ids, "join", 1, "M1")
	pathOf(t, tree, ids, "R1", 0)
}

func TestBuildContextTreeSiblingContextsMergeRejected(t *testing.T) {
	// A block combining outputs of two *unclosed* sibling maps is illegal.
	p, manifests, g, _ := buildContextTreeFixture(t, []ctBlock{
		{"Ma", BlockKindMap, nil},
		{"Mb", BlockKindMap, nil},
		{"bad", BlockKindStandard, []string{"Ma", "Mb"}},
	})

	_, err := BuildContextTree(p, manifests, g)
	if err == nil {
		t.Fatal("expected error for block straddling sibling map contexts")
	}
	if !strings.Contains(err.Error(), "incompatible map contexts") {
		t.Errorf("unexpected error text: %v", err)
	}
}

func TestBuildContextTreeMaxDepthRejected(t *testing.T) {
	// MaxMapDepth+1 nested maps must be rejected.
	blocks := []ctBlock{{"M0", BlockKindMap, nil}}
	prev := "M0"
	for i := 1; i <= MaxMapDepth; i++ {
		name := "M" + strings.Repeat("x", i)
		blocks = append(blocks, ctBlock{name, BlockKindMap, []string{prev}})
		prev = name
	}
	blocks = append(blocks, ctBlock{"deep", BlockKindStandard, []string{prev}})
	p, manifests, g, _ := buildContextTreeFixture(t, blocks)

	_, err := BuildContextTree(p, manifests, g)
	if err == nil {
		t.Fatal("expected error for exceeding MaxMapDepth")
	}
	if !strings.Contains(err.Error(), "maximum supported depth") {
		t.Errorf("unexpected error text: %v", err)
	}
}

func TestBuildContextTreeSequentialPairs(t *testing.T) {
	// Two sequential (non-nested) map/reduce pairs remain legal and both
	// run at the top level.
	p, manifests, g, ids := buildContextTreeFixture(t, []ctBlock{
		{"Ma", BlockKindMap, nil},
		{"Xa", BlockKindStandard, []string{"Ma"}},
		{"Ra", BlockKindReduce, []string{"Xa"}},
		{"Mb", BlockKindMap, []string{"Ra"}},
		{"Xb", BlockKindStandard, []string{"Mb"}},
		{"Rb", BlockKindReduce, []string{"Xb"}},
	})

	tree, err := BuildContextTree(p, manifests, g)
	if err != nil {
		t.Fatalf("BuildContextTree: %v", err)
	}
	pathOf(t, tree, ids, "Ma", 0)
	pathOf(t, tree, ids, "Ra", 0)
	pathOf(t, tree, ids, "Mb", 0)
	pathOf(t, tree, ids, "Rb", 0)
	pathOf(t, tree, ids, "Xa", 1, "Ma")
	pathOf(t, tree, ids, "Xb", 1, "Mb")
}

func TestBuildContextTreeBroadcastIntoNested(t *testing.T) {
	// A depth-0 model and a depth-1 reference both feed a depth-2 block.
	p, manifests, g, ids := buildContextTreeFixture(t, []ctBlock{
		{"model", BlockKindStandard, nil},
		{"M1", BlockKindMap, nil},
		{"ref", BlockKindStandard, []string{"M1"}},
		{"M2", BlockKindMap, []string{"M1"}},
		{"X", BlockKindStandard, []string{"M2", "ref", "model"}},
		{"R2", BlockKindReduce, []string{"X"}},
		{"R1", BlockKindReduce, []string{"R2"}},
	})

	tree, err := BuildContextTree(p, manifests, g)
	if err != nil {
		t.Fatalf("BuildContextTree: %v", err)
	}
	pathOf(t, tree, ids, "model", 0)
	pathOf(t, tree, ids, "ref", 1, "M1")
	pathOf(t, tree, ids, "X", 2, "M1", "M2")
}

func TestBuildContextTreeReduceOutsideContext(t *testing.T) {
	// A reduce consuming a plain (non-mapped) collection output is legal
	// and runs as a standard top-level block.
	p, manifests, g, ids := buildContextTreeFixture(t, []ctBlock{
		{"src", BlockKindStandard, nil},
		{"R", BlockKindReduce, []string{"src"}},
	})

	tree, err := BuildContextTree(p, manifests, g)
	if err != nil {
		t.Fatalf("BuildContextTree: %v", err)
	}
	pathOf(t, tree, ids, "R", 0)
	if len(tree.Reduces) != 0 {
		t.Errorf("no context should be closed, got %v", tree.Reduces)
	}
}
