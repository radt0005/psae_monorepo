package spade

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// outputEntry is an internal type for named outputs.
type outputEntry struct {
	name  string
	value IntoOutput
}

// Outputs is a collection of named outputs for handlers that produce multiple results.
type Outputs struct {
	entries []outputEntry
}

// NewOutputs creates an empty Outputs collection.
func NewOutputs() *Outputs {
	return &Outputs{}
}

// Add appends a named output to the collection.
func (o *Outputs) Add(name string, value IntoOutput) {
	o.entries = append(o.entries, outputEntry{name: name, value: value})
}

// WriteTo writes all entries to subdirectories under outputDir.
func (o *Outputs) WriteTo(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	for _, e := range o.entries {
		dir := filepath.Join(outputDir, e.name)
		if err := e.value.WriteTo(dir); err != nil {
			return err
		}
	}
	return nil
}

// DefaultOutputName returns a sentinel value for internal routing.
func (o *Outputs) DefaultOutputName() string { return "__outputs__" }

// TypeName returns a sentinel value for internal routing.
func (o *Outputs) TypeName() string { return "__outputs__" }

// ManifestEntry returns a sentinel ManifestInfo.
func (o *Outputs) ManifestEntry() ManifestInfo {
	return ManifestInfo{TypeName: "__outputs__"}
}

// unitOutput is an internal type for handlers that return no output.
type unitOutput struct{}

func (u unitOutput) WriteTo(_ string) error    { return nil }
func (u unitOutput) DefaultOutputName() string { return "__none__" }
func (u unitOutput) TypeName() string          { return "__none__" }
func (u unitOutput) ManifestEntry() ManifestInfo {
	return ManifestInfo{TypeName: "__none__"}
}

// ReadBlockManifest reads the block manifest to get output declarations.
// Checks SPADE_BLOCK_MANIFEST env var first, then block.yaml in base dir.
// Returns nil if no manifest found.
func ReadBlockManifest(base string) map[string]any {
	// Check env var first
	if envPath := os.Getenv("SPADE_BLOCK_MANIFEST"); envPath != "" {
		if m := readManifestOutputs(envPath); m != nil {
			return m
		}
	}

	// Check block.yaml in base dir
	blockYaml := filepath.Join(base, "block.yaml")
	if m := readManifestOutputs(blockYaml); m != nil {
		return m
	}

	return nil
}

func readManifestOutputs(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var manifest map[string]any
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil
	}
	outputs, ok := manifest["outputs"]
	if !ok {
		return nil
	}
	if m, ok := outputs.(map[string]any); ok {
		return m
	}
	return nil
}

// WriteOutputs writes handler output(s) to <base>/outputs/.
func WriteOutputs(result IntoOutput, base string, manifestOutputs map[string]any) error {
	outputName := result.DefaultOutputName()
	outputsRoot := filepath.Join(base, "outputs")

	// Outputs collection handles its own subdirectories
	if outputName == "__outputs__" {
		return result.WriteTo(outputsRoot)
	}

	// No-output handler
	if outputName == "__none__" {
		return nil
	}

	// Single output: determine output directory name
	name := outputName
	if manifestOutputs != nil && len(manifestOutputs) == 1 {
		for k := range manifestOutputs {
			name = k
			break
		}
	}

	return result.WriteTo(filepath.Join(outputsRoot, name))
}
