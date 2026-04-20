package core

import (
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// --- Phase 2.1: Dependency Graph ---

// DependencyGraph represents the DAG of block dependencies in a pipeline.
type DependencyGraph struct {
	// Forward: block UUID -> list of dependent block UUIDs (blocks that depend on this one)
	Forward map[uuid.UUID][]uuid.UUID
	// Reverse: block UUID -> list of dependency UUIDs (blocks this one depends on)
	Reverse map[uuid.UUID][]uuid.UUID
	// AllBlocks: set of all block UUIDs in the graph
	AllBlocks map[uuid.UUID]bool
}

// BuildDependencyGraph constructs the dependency graph from a pipeline's block input references.
func BuildDependencyGraph(pipeline Pipeline) (DependencyGraph, error) {
	g := DependencyGraph{
		Forward:   make(map[uuid.UUID][]uuid.UUID),
		Reverse:   make(map[uuid.UUID][]uuid.UUID),
		AllBlocks: make(map[uuid.UUID]bool),
	}

	// Register all blocks
	for _, block := range pipeline.Blocks {
		g.AllBlocks[block.Id] = true
	}

	// Build edges from input references
	for _, block := range pipeline.Blocks {
		for _, input := range block.Inputs {
			var depID uuid.UUID
			if input.Block != nil {
				depID = *input.Block
			} else {
				depID = input.ID
			}

			if depID == uuid.Nil {
				continue
			}

			if !g.AllBlocks[depID] {
				return g, fmt.Errorf("block %s references unknown dependency %s", block.Id, depID)
			}

			g.Forward[depID] = append(g.Forward[depID], block.Id)
			g.Reverse[block.Id] = append(g.Reverse[block.Id], depID)
		}
	}

	return g, nil
}

// TopologicalSort returns a valid execution order using Kahn's algorithm.
// Returns an error if a cycle is detected.
func (g *DependencyGraph) TopologicalSort() ([]uuid.UUID, error) {
	// Compute in-degree for each node
	inDegree := make(map[uuid.UUID]int)
	for id := range g.AllBlocks {
		inDegree[id] = len(g.Reverse[id])
	}

	// Start with nodes that have no incoming edges
	var queue []uuid.UUID
	for id := range g.AllBlocks {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []uuid.UUID
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)

		for _, dep := range g.Forward[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(sorted) != len(g.AllBlocks) {
		return nil, fmt.Errorf("cycle detected in dependency graph")
	}

	return sorted, nil
}

// SourceBlocks returns all blocks with no incoming edges (no dependencies).
func (g *DependencyGraph) SourceBlocks() []uuid.UUID {
	var sources []uuid.UUID
	for id := range g.AllBlocks {
		if len(g.Reverse[id]) == 0 {
			sources = append(sources, id)
		}
	}
	return sources
}

// DownstreamBlocks returns all blocks that directly depend on the given block.
func (g *DependencyGraph) DownstreamBlocks(id uuid.UUID) []uuid.UUID {
	return g.Forward[id]
}

// --- Phase 2.2: Input Resolution ---

// ResolvedInput describes a fully resolved input mapping.
type ResolvedInput struct {
	InputName      string
	SourceBlockID  uuid.UUID
	SourceOutputName string
	SourceOutputDecl OutputDeclaration
}

// typesCompatible checks whether a dependency output type can satisfy a block
// input type.  In addition to exact matches it handles two map/reduce cases:
//   - An "expansion" output (from a map block) satisfies a "file" or
//     "directory" input because the scheduler delivers individual items.
//   - A "file" output satisfies a "collection" input on a reduce block
//     because the scheduler gathers N mapped outputs into a collection.
func typesCompatible(inputType, outputType string) bool {
	if inputType == outputType {
		return true
	}
	// Map expansion items resolve to files (or directories) at runtime.
	// When a reduce block directly follows a map block the scheduler
	// gathers expansion items into a collection.
	if outputType == "expansion" && (inputType == "file" || inputType == "directory" || inputType == "collection") {
		return true
	}
	// In a map→reduce chain the last mapped block outputs single files
	// that the scheduler collects into a collection for the reduce block.
	if outputType == "file" && inputType == "collection" {
		return true
	}
	return false
}

// ResolveInputs implements the 4-step resolution algorithm from pipeline.md section 5.4.
func ResolveInputs(block PipelineBlock, dependencies map[uuid.UUID]BlockManifest, currentManifest BlockManifest) (map[string]ResolvedInput, error) {
	resolved := make(map[string]ResolvedInput)
	matchedInputs := make(map[string]bool)     // inputs on current block already matched
	matchedOutputs := make(map[string]bool)     // "blockID:outputName" already matched

	// Step 1: Resolve explicit references
	for _, ref := range block.Inputs {
		if ref.Block == nil {
			continue // bare reference, handled in step 2
		}
		depManifest, ok := dependencies[*ref.Block]
		if !ok {
			return nil, fmt.Errorf("explicit reference to unknown block %s", ref.Block)
		}
		outputDecl, ok := depManifest.Outputs[ref.Output]
		if !ok {
			return nil, fmt.Errorf("block %s has no output named %q", ref.Block, ref.Output)
		}

		// Find matching input.  If the reference names a downstream input
		// via `as`, use it directly (after type-checking).  Otherwise
		// fall back to the first unmatched type-compatible input, scanned
		// in a deterministic order.
		var matchedInput string
		if ref.As != "" {
			inputDecl, exists := currentManifest.Inputs[ref.As]
			if !exists {
				return nil, fmt.Errorf("explicit reference targets unknown input %q", ref.As)
			}
			if !typesCompatible(inputDecl.Type, outputDecl.Type) {
				return nil, fmt.Errorf("explicit reference to %q: type %q not compatible with output type %q",
					ref.As, inputDecl.Type, outputDecl.Type)
			}
			matchedInput = ref.As
		} else {
			names := make([]string, 0, len(currentManifest.Inputs))
			for n := range currentManifest.Inputs {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, inputName := range names {
				if matchedInputs[inputName] {
					continue
				}
				if typesCompatible(currentManifest.Inputs[inputName].Type, outputDecl.Type) {
					matchedInput = inputName
					break
				}
			}
		}
		if matchedInput == "" {
			return nil, fmt.Errorf("no unmatched input of type %q for explicit reference to %s.%s",
				outputDecl.Type, ref.Block, ref.Output)
		}

		resolved[matchedInput] = ResolvedInput{
			InputName:        matchedInput,
			SourceBlockID:    *ref.Block,
			SourceOutputName: ref.Output,
			SourceOutputDecl: outputDecl,
		}
		matchedInputs[matchedInput] = true
		key := fmt.Sprintf("%s:%s", ref.Block, ref.Output)
		matchedOutputs[key] = true
	}

	// Step 2: Resolve bare references by type matching
	for _, ref := range block.Inputs {
		if ref.Block != nil {
			continue // explicit reference, already handled
		}
		depManifest, ok := dependencies[ref.ID]
		if !ok {
			return nil, fmt.Errorf("bare reference to unknown block %s", ref.ID)
		}

		// For each unmatched output of the dependency, try to match to unmatched inputs
		for outputName, outputDecl := range depManifest.Outputs {
			key := fmt.Sprintf("%s:%s", ref.ID, outputName)
			if matchedOutputs[key] {
				continue
			}

			var candidates []string
			for inputName, inputDecl := range currentManifest.Inputs {
				if matchedInputs[inputName] {
					continue
				}
				if typesCompatible(inputDecl.Type, outputDecl.Type) {
					candidates = append(candidates, inputName)
				}
			}

			// Step 3: Check for ambiguity
			if len(candidates) > 1 {
				return nil, fmt.Errorf("ambiguous type match: output %s.%s (type %q) matches multiple inputs: %v",
					ref.ID, outputName, outputDecl.Type, candidates)
			}

			if len(candidates) == 1 {
				resolved[candidates[0]] = ResolvedInput{
					InputName:        candidates[0],
					SourceBlockID:    ref.ID,
					SourceOutputName: outputName,
					SourceOutputDecl: outputDecl,
				}
				matchedInputs[candidates[0]] = true
				matchedOutputs[key] = true
			}
		}
	}

	// Step 4: Check for completeness - every non-scalar input must be matched
	for inputName, inputDecl := range currentManifest.Inputs {
		if matchedInputs[inputName] {
			continue
		}
		// Scalar inputs are provided via params.yaml, not from dependencies
		switch inputDecl.Type {
		case "string", "number", "boolean":
			continue
		}
		return nil, fmt.Errorf("input %q (type %q) has no matching source", inputName, inputDecl.Type)
	}

	return resolved, nil
}

// --- Phase 2.3: Pipeline Validation ---

// ValidatePipeline performs all validation checks from pipeline.md section 7.
func ValidatePipeline(pipeline Pipeline, manifests map[string]BlockManifest) []error {
	var errs []error

	// 1. All block id values are unique
	idSet := make(map[uuid.UUID]bool)
	for _, block := range pipeline.Blocks {
		if idSet[block.Id] {
			errs = append(errs, fmt.Errorf("duplicate block id: %s", block.Id))
		}
		idSet[block.Id] = true
	}

	// 2. All invocation IDs referenced in inputs exist in the pipeline's block list
	for _, block := range pipeline.Blocks {
		for _, input := range block.Inputs {
			var refID uuid.UUID
			if input.Block != nil {
				refID = *input.Block
			} else {
				refID = input.ID
			}
			if refID != uuid.Nil && !idSet[refID] {
				errs = append(errs, fmt.Errorf("block %s references non-existent block %s", block.Id, refID))
			}
		}
	}

	// 3. All name values refer to known block types
	for _, block := range pipeline.Blocks {
		if _, ok := manifests[block.Name]; !ok {
			errs = append(errs, fmt.Errorf("block %s references unknown block type %q", block.Id, block.Name))
		}
	}

	// 4. The dependency graph is acyclic
	graph, err := BuildDependencyGraph(pipeline)
	if err != nil {
		errs = append(errs, err)
		return errs // can't continue without a valid graph
	}
	if _, err := graph.TopologicalSort(); err != nil {
		errs = append(errs, err)
	}

	// 5 & 6. Input/output type compatibility and named output references
	for _, block := range pipeline.Blocks {
		currentManifest, ok := manifests[block.Name]
		if !ok {
			continue // already reported in check 3
		}

		depManifests := make(map[uuid.UUID]BlockManifest)
		for _, input := range block.Inputs {
			var depID uuid.UUID
			if input.Block != nil {
				depID = *input.Block
			} else {
				depID = input.ID
			}
			if depID == uuid.Nil {
				continue
			}
			// Find the dependency block's name to look up its manifest
			for _, depBlock := range pipeline.Blocks {
				if depBlock.Id == depID {
					if m, mok := manifests[depBlock.Name]; mok {
						depManifests[depID] = m
					}
					break
				}
			}
		}

		if _, err := ResolveInputs(block, depManifests, currentManifest); err != nil {
			errs = append(errs, fmt.Errorf("block %s (%s): %w", block.Id, block.Name, err))
		}
	}

	// 7. All required args - checked via manifest input declarations for scalar types
	for _, block := range pipeline.Blocks {
		manifest, ok := manifests[block.Name]
		if !ok {
			continue
		}
		for inputName, inputDecl := range manifest.Inputs {
			switch inputDecl.Type {
			case "string", "number", "boolean":
				if block.Args == nil {
					errs = append(errs, fmt.Errorf("block %s missing required arg %q", block.Id, inputName))
				} else if _, exists := block.Args[inputName]; !exists {
					errs = append(errs, fmt.Errorf("block %s missing required arg %q", block.Id, inputName))
				}
			}
		}
	}

	// Map/reduce validation (pipeline.md section 8.3)
	errs = append(errs, validateMapReduce(pipeline, manifests, graph)...)

	return errs
}

// validateMapReduce checks map/reduce-specific validation rules.
func validateMapReduce(pipeline Pipeline, manifests map[string]BlockManifest, graph DependencyGraph) []error {
	var errs []error

	// Build lookup from ID to block for convenience
	blockByID := make(map[uuid.UUID]PipelineBlock)
	for _, b := range pipeline.Blocks {
		blockByID[b.Id] = b
	}

	for _, block := range pipeline.Blocks {
		manifest, ok := manifests[block.Name]
		if !ok {
			continue
		}

		if manifest.Kind == BlockKindMap {
			// 1. Map block must output expansion type
			hasExpansion := false
			for _, out := range manifest.Outputs {
				if out.Type == "expansion" {
					hasExpansion = true
					break
				}
			}
			if !hasExpansion {
				errs = append(errs, fmt.Errorf("map block %s (%s) must have an expansion output", block.Id, block.Name))
			}

			// 2. Map block must eventually be followed by a reduce block
			if !hasDownstreamReduce(block.Id, graph, blockByID, manifests, make(map[uuid.UUID]bool)) {
				errs = append(errs, fmt.Errorf("map block %s (%s) has no downstream reduce block", block.Id, block.Name))
			}

			// 3. No nested maps: check if any downstream map block appears before a reduce
			if hasNestedMap(block.Id, graph, blockByID, manifests) {
				errs = append(errs, fmt.Errorf("map block %s (%s) has a nested map before its reduce", block.Id, block.Name))
			}
		}

		if manifest.Kind == BlockKindReduce {
			// 4. Reduce blocks must accept a collection input
			hasCollection := false
			for _, in := range manifest.Inputs {
				if in.Type == "collection" {
					hasCollection = true
					break
				}
			}
			if !hasCollection {
				errs = append(errs, fmt.Errorf("reduce block %s (%s) must have a collection input", block.Id, block.Name))
			}
		}
	}

	return errs
}

// hasDownstreamReduce checks if there's a reduce block reachable from the given block.
func hasDownstreamReduce(id uuid.UUID, graph DependencyGraph, blocks map[uuid.UUID]PipelineBlock, manifests map[string]BlockManifest, visited map[uuid.UUID]bool) bool {
	if visited[id] {
		return false
	}
	visited[id] = true

	for _, downstream := range graph.Forward[id] {
		block, ok := blocks[downstream]
		if !ok {
			continue
		}
		manifest, ok := manifests[block.Name]
		if !ok {
			continue
		}
		if manifest.Kind == BlockKindReduce {
			return true
		}
		if hasDownstreamReduce(downstream, graph, blocks, manifests, visited) {
			return true
		}
	}
	return false
}

// hasNestedMap checks if another map block appears between this map and its reduce.
func hasNestedMap(mapID uuid.UUID, graph DependencyGraph, blocks map[uuid.UUID]PipelineBlock, manifests map[string]BlockManifest) bool {
	visited := make(map[uuid.UUID]bool)
	return checkNestedMap(mapID, mapID, graph, blocks, manifests, visited)
}

func checkNestedMap(startID, currentID uuid.UUID, graph DependencyGraph, blocks map[uuid.UUID]PipelineBlock, manifests map[string]BlockManifest, visited map[uuid.UUID]bool) bool {
	if visited[currentID] {
		return false
	}
	visited[currentID] = true

	for _, downstream := range graph.Forward[currentID] {
		block, ok := blocks[downstream]
		if !ok {
			continue
		}
		manifest, ok := manifests[block.Name]
		if !ok {
			continue
		}
		if manifest.Kind == BlockKindReduce {
			continue // stop at reduce boundary
		}
		if manifest.Kind == BlockKindMap && downstream != startID {
			return true // nested map found
		}
		if checkNestedMap(startID, downstream, graph, blocks, manifests, visited) {
			return true
		}
	}
	return false
}
