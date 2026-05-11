package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// ErrInvalidLockfile signals that a lockfile exists but is malformed or
// contains invalid bindings.  Callers should surface this with a
// "delete the lockfile to regenerate" suggestion per pipeline.md §6.6.
var ErrInvalidLockfile = errors.New("invalid lockfile")

// Lockfile maps short codes to UUIDv7 invocation IDs for a pipeline.
// See spec/pipeline.md §6.3 for the on-disk format.
type Lockfile struct {
	Pipeline string               `yaml:"pipeline,omitempty"`
	Version  string               `yaml:"version,omitempty"`
	Bindings map[string]uuid.UUID `yaml:"bindings"`
}

// LockfilePathFor returns the sibling lockfile path for a pipeline file.
// For "pipeline.yaml" it returns "pipeline.lock.yaml"; for "foo.yml" it
// returns "foo.lock.yaml".  Directory components are preserved.
func LockfilePathFor(pipelinePath string) string {
	dir := filepath.Dir(pipelinePath)
	base := filepath.Base(pipelinePath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, stem+".lock.yaml")
}

// LoadLockfile reads and parses a lockfile.  A missing file is not an
// error: the returned Lockfile is zero-valued with a non-nil empty
// Bindings map, signaling "no bindings yet."  Malformed YAML or invalid
// UUIDs return ErrInvalidLockfile wrapped with detail.
func LoadLockfile(path string) (Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Lockfile{Bindings: make(map[string]uuid.UUID)}, nil
		}
		return Lockfile{}, fmt.Errorf("reading lockfile %s: %w", path, err)
	}

	// Decode into a string-valued map first so we can wrap UUID-parse
	// failures in ErrInvalidLockfile rather than letting yaml.v3 surface
	// a less actionable decode error.
	var raw struct {
		Pipeline string            `yaml:"pipeline"`
		Version  string            `yaml:"version"`
		Bindings map[string]string `yaml:"bindings"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Lockfile{}, fmt.Errorf("%w: parsing %s: %v", ErrInvalidLockfile, path, err)
	}

	lock := Lockfile{
		Pipeline: raw.Pipeline,
		Version:  raw.Version,
		Bindings: make(map[string]uuid.UUID, len(raw.Bindings)),
	}
	for code, idStr := range raw.Bindings {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return Lockfile{}, fmt.Errorf("%w: binding %q in %s is not a valid UUID: %v",
				ErrInvalidLockfile, code, path, err)
		}
		lock.Bindings[code] = id
	}
	return lock, nil
}

// SaveLockfile writes a Lockfile to disk with bindings sorted by short
// code so that the file diffs cleanly in version control.
func SaveLockfile(lock Lockfile, path string) error {
	// Sort keys for deterministic output.
	codes := make([]string, 0, len(lock.Bindings))
	for code := range lock.Bindings {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	// Build a yaml.Node tree manually so we can guarantee key ordering.
	// (yaml.v3 emits map keys in iteration order for maps but in struct
	// field order for structs; building the node tree gives us full
	// control.)
	root := &yaml.Node{Kind: yaml.MappingNode}
	if lock.Pipeline != "" {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "pipeline"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: lock.Pipeline},
		)
	}
	if lock.Version != "" {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "version"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: lock.Version, Style: yaml.DoubleQuotedStyle},
		)
	}
	bindingsNode := &yaml.Node{Kind: yaml.MappingNode}
	for _, code := range codes {
		bindingsNode.Content = append(bindingsNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: code, Style: yaml.DoubleQuotedStyle},
			&yaml.Node{Kind: yaml.ScalarNode, Value: lock.Bindings[code].String()},
		)
	}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "bindings"},
		bindingsNode,
	)

	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshaling lockfile: %w", err)
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory for lockfile: %w", err)
		}
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing lockfile %s: %w", path, err)
	}
	return nil
}
