package core

import (
	"bytes"
	"fmt"
	"os"
	"regexp"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// ShortCodePattern matches a valid short code: `@<identifier>` where
// `<identifier>` is `[A-Za-z_][A-Za-z0-9_]*`.  See pipeline.md §6.1.
const ShortCodePattern = `^@[A-Za-z_][A-Za-z0-9_]*$`

var shortCodeRegex = regexp.MustCompile(ShortCodePattern)

// isShortCode reports whether s is a syntactically valid short code.
func isShortCode(s string) bool {
	return shortCodeRegex.MatchString(s)
}

// ResolveShortCodes walks a parsed pipeline YAML document and substitutes
// short codes with UUIDs, consulting (and possibly extending) the provided
// lockfile.  The walk is structure-aware: it visits only `blocks[i].id`,
// bare `inputs[j]` scalars, and `inputs[j].block` scalars.  It never
// descends into `args`, `outputs`, `description`, or the pipeline-level
// `id`.
//
// `root` should be the content node of the pipeline document -- i.e. the
// top-level mapping node (not the document node).
//
// Returns whether the lockfile's Bindings map was mutated, so the caller
// can decide whether to persist it.  A repeated short code in the same
// pipeline binds once: the second occurrence reuses the first binding
// (rather than minting a fresh UUID), which is what lets the existing
// duplicate-id validation surface authoring mistakes correctly.
func ResolveShortCodes(root *yaml.Node, lock *Lockfile) (changed bool, err error) {
	if root == nil {
		return false, fmt.Errorf("nil pipeline root")
	}
	if lock.Bindings == nil {
		lock.Bindings = make(map[string]uuid.UUID)
	}
	if root.Kind != yaml.MappingNode {
		return false, fmt.Errorf("pipeline root must be a YAML mapping, got kind %d", root.Kind)
	}

	// Disallow a short code on the pipeline-level `id` (spec §6.1).
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		val := root.Content[i+1]
		if key.Value == "id" && val.Kind == yaml.ScalarNode && isShortCode(val.Value) {
			return false, fmt.Errorf("short code %q is not allowed on the pipeline-level `id` field (spec/pipeline.md §6.1): omit `id` to let the CLI generate one at run/submission time, or use a concrete UUID",
				val.Value)
		}
	}

	// Find the `blocks` sequence and walk each block entry.
	blocksNode := mappingValue(root, "blocks")
	if blocksNode == nil {
		return false, nil // no blocks → nothing to resolve
	}
	if blocksNode.Kind != yaml.SequenceNode {
		return false, fmt.Errorf("`blocks` must be a sequence, got kind %d", blocksNode.Kind)
	}

	resolve := func(code string) (string, error) {
		if id, ok := lock.Bindings[code]; ok {
			return id.String(), nil
		}
		id, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("generating UUIDv7 for %s: %w", code, err)
		}
		lock.Bindings[code] = id
		changed = true
		return id.String(), nil
	}

	for _, block := range blocksNode.Content {
		if block.Kind != yaml.MappingNode {
			continue
		}

		// Walk this block's id field.
		if idNode := mappingValue(block, "id"); idNode != nil && idNode.Kind == yaml.ScalarNode {
			if isShortCode(idNode.Value) {
				uuidStr, err := resolve(idNode.Value)
				if err != nil {
					return changed, err
				}
				idNode.Value = uuidStr
				idNode.Tag = ""    // let yaml.v3 infer the string tag
				idNode.Style = 0   // emit unquoted
			}
		}

		// Walk this block's inputs sequence.
		inputsNode := mappingValue(block, "inputs")
		if inputsNode == nil || inputsNode.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range inputsNode.Content {
			switch item.Kind {
			case yaml.ScalarNode:
				// Bare reference.
				if isShortCode(item.Value) {
					uuidStr, err := resolve(item.Value)
					if err != nil {
						return changed, err
					}
					item.Value = uuidStr
					item.Tag = ""
					item.Style = 0
				}
			case yaml.MappingNode:
				// Explicit reference: replace only the `block` scalar.
				if blockRef := mappingValue(item, "block"); blockRef != nil && blockRef.Kind == yaml.ScalarNode {
					if isShortCode(blockRef.Value) {
						uuidStr, err := resolve(blockRef.Value)
						if err != nil {
							return changed, err
						}
						blockRef.Value = uuidStr
						blockRef.Tag = ""
						blockRef.Style = 0
					}
				}
			}
		}

		// Explicitly do NOT descend into `args`, `outputs`, `name`, or
		// any other field.  Short codes in args are user data per §6.1
		// and must pass through verbatim.
	}

	return changed, nil
}

// mappingValue returns the value node for a given key within a mapping
// node, or nil if the key is absent.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// LoadAndResolvePipeline reads a pipeline file, resolves any short codes
// using the sibling lockfile (creating or updating it as needed), and
// returns the parsed Pipeline along with the (possibly updated) Lockfile
// and a flag indicating whether the lockfile was persisted.
//
// This is the entry point CLI commands should use for any pipeline that
// may contain short codes.  The existing LoadPipeline remains valid for
// pipelines known to be in resolved (UUID-only) form.
func LoadAndResolvePipeline(pipelinePath string) (Pipeline, Lockfile, bool, error) {
	data, err := os.ReadFile(pipelinePath)
	if err != nil {
		return Pipeline{}, Lockfile{}, false, fmt.Errorf("reading pipeline %s: %w", pipelinePath, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Pipeline{}, Lockfile{}, false, fmt.Errorf("parsing pipeline %s: %w", pipelinePath, err)
	}

	// doc is a DocumentNode wrapping the root mapping.
	if len(doc.Content) == 0 {
		return Pipeline{}, Lockfile{}, false, fmt.Errorf("pipeline %s is empty", pipelinePath)
	}
	root := doc.Content[0]

	lockPath := LockfilePathFor(pipelinePath)
	lock, err := LoadLockfile(lockPath)
	if err != nil {
		return Pipeline{}, Lockfile{}, false, err
	}

	changed, err := ResolveShortCodes(root, &lock)
	if err != nil {
		return Pipeline{}, Lockfile{}, false, err
	}

	wrotelockfile := false
	if changed {
		if err := SaveLockfile(lock, lockPath); err != nil {
			return Pipeline{}, Lockfile{}, false, fmt.Errorf("writing lockfile: %w", err)
		}
		wrotelockfile = true
	}

	// Re-marshal the (possibly mutated) document and unmarshal into the
	// existing Pipeline struct.  We marshal back to bytes rather than
	// decoding the node tree directly so that the existing typed parse
	// path (with its InputRef custom unmarshaler) is fully reused.
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return Pipeline{}, Lockfile{}, false, fmt.Errorf("re-marshaling resolved pipeline: %w", err)
	}
	enc.Close()

	var pipeline Pipeline
	if err := yaml.Unmarshal(buf.Bytes(), &pipeline); err != nil {
		return Pipeline{}, Lockfile{}, false, fmt.Errorf("parsing resolved pipeline: %w", err)
	}

	return pipeline, lock, wrotelockfile, nil
}
