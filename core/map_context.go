package core

import (
	"fmt"

	"github.com/google/uuid"
)

// MaxMapDepth is the maximum number of map contexts a block may be nested
// under.  Invocation counts multiply per level (N₁×N₂×…), so a hard cap
// turns a pathological pipeline into an authoring-time error instead of a
// scheduler meltdown.
const MaxMapDepth = 4

// ContextPath is the chain of enclosing map block IDs for a block,
// outermost first.  len(path) is the depth at which the block's
// invocations run — i.e. the length of their map index vector.
//
// A map block's own path is that of its *enclosing* context (the map
// block itself is a mapped block of its parent); blocks strictly between
// a map and its reduce carry the map's ID as their last path element; a
// reduce block's path is the closed context's path minus that last
// element.
type ContextPath []uuid.UUID

// Equal reports whether two context paths are identical.
func (p ContextPath) Equal(o ContextPath) bool {
	if len(p) != len(o) {
		return false
	}
	for i := range p {
		if p[i] != o[i] {
			return false
		}
	}
	return true
}

// IsPrefixOf reports whether p is a (non-strict) prefix of o.
func (p ContextPath) IsPrefixOf(o ContextPath) bool {
	if len(p) > len(o) {
		return false
	}
	for i := range p {
		if p[i] != o[i] {
			return false
		}
	}
	return true
}

// ContextTree is the static map/reduce structure of a pipeline: every
// block's context path, plus per-map-context membership and the reduce
// block(s) that close each context.  It is computed once from the
// pipeline and manifests and shared by validation (`spade check`), the
// scheduler (fan-out and reduce readiness), and the worker (input
// resolution).
type ContextTree struct {
	// Paths maps every block to its context path.
	Paths map[uuid.UUID]ContextPath
	// Members maps a map block ID to the set of blocks that run inside
	// its context at its depth: blocks whose path's last element is that
	// map.  This includes nested map blocks and the reduces that close
	// nested contexts, but not the nested contexts' interiors (those are
	// members of the nested map).
	Members map[uuid.UUID]map[uuid.UUID]bool
	// Reduces maps a map block ID to the reduce block(s) that close its
	// context.
	Reduces map[uuid.UUID][]uuid.UUID
}

// Depth returns the map depth at which a block's invocations run
// (0 = not mapped).
func (t *ContextTree) Depth(id uuid.UUID) int {
	return len(t.Paths[id])
}

// BuildContextTree assigns every block in the pipeline its context path by
// walking the DAG in topological order, and derives context membership.
// For each edge A→B, A "offers" B a path: A's own path, extended by A
// itself when A is a map block.  A non-reduce block adopts the longest
// offered path; a reduce block adopts the longest offered path minus its
// last element (closing the innermost context).  All other offers must be
// prefixes of the longest — those are broadcasts from enclosing contexts.
//
// Well-nestedness violations detected here:
//   - two offers where neither is a prefix of the other (a block cannot
//     belong to two sibling map contexts — e.g. two maps merging into one
//     downstream block or reduce)
//   - nesting deeper than MaxMapDepth
//
// A reduce block whose offers are all empty is not closing any context; it
// is treated as a standard block at the top level (a reduce consuming a
// plain collection output is legal).
func BuildContextTree(p Pipeline, manifests map[string]BlockManifest, g DependencyGraph) (*ContextTree, error) {
	order, err := g.TopologicalSort()
	if err != nil {
		return nil, err
	}

	blockByID := make(map[uuid.UUID]PipelineBlock, len(p.Blocks))
	for _, b := range p.Blocks {
		blockByID[b.Id] = b
	}
	kindOf := func(id uuid.UUID) BlockKind {
		b, ok := blockByID[id]
		if !ok {
			return BlockKindStandard
		}
		m, ok := manifests[b.Name]
		if !ok {
			return BlockKindStandard
		}
		return m.Kind
	}
	label := func(id uuid.UUID) string {
		if b, ok := blockByID[id]; ok {
			return fmt.Sprintf("%s (%s)", id, b.Name)
		}
		return id.String()
	}

	tree := &ContextTree{
		Paths:   make(map[uuid.UUID]ContextPath, len(p.Blocks)),
		Members: make(map[uuid.UUID]map[uuid.UUID]bool),
		Reduces: make(map[uuid.UUID][]uuid.UUID),
	}
	// Every map block owns a (possibly empty) context.
	for _, b := range p.Blocks {
		if kindOf(b.Id) == BlockKindMap {
			tree.Members[b.Id] = make(map[uuid.UUID]bool)
		}
	}

	for _, id := range order {
		// Gather the offered path from each dependency.
		var longest ContextPath
		var longestFrom uuid.UUID
		first := true
		offers := make(map[uuid.UUID]ContextPath)
		for _, dep := range g.Reverse[id] {
			offered := tree.Paths[dep]
			if kindOf(dep) == BlockKindMap {
				offered = append(append(ContextPath{}, offered...), dep)
			}
			offers[dep] = offered
			if first || len(offered) > len(longest) {
				longest = offered
				longestFrom = dep
				first = false
			}
		}

		// Every other offer must be a prefix of the longest — anything
		// else means the block straddles sibling contexts.
		for dep, offered := range offers {
			if !offered.IsPrefixOf(longest) {
				return nil, fmt.Errorf(
					"block %s receives inputs from incompatible map contexts (via %s and %s); a block cannot combine outputs of two maps that are not closed by reduces — route one side through its reduce first",
					label(id), label(dep), label(longestFrom))
			}
		}

		path := longest
		if kindOf(id) == BlockKindReduce && len(longest) > 0 {
			// The reduce closes the innermost context it consumes from:
			// it runs at the parent depth.
			closed := longest[len(longest)-1]
			path = longest[:len(longest)-1]
			tree.Reduces[closed] = append(tree.Reduces[closed], id)
		}

		if len(path) > MaxMapDepth {
			return nil, fmt.Errorf(
				"block %s is nested %d map contexts deep; the maximum supported depth is %d",
				label(id), len(path), MaxMapDepth)
		}

		tree.Paths[id] = path
		if len(path) > 0 {
			owner := path[len(path)-1]
			tree.Members[owner][id] = true
		}
	}

	return tree, nil
}
