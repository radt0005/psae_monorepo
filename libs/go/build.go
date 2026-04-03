package spade

// manifestField holds a named manifest entry for an input or output.
type manifestField struct {
	name string
	info ManifestInfo
}

// ManifestBuilder provides a fluent API for generating block manifest dictionaries.
type ManifestBuilder struct {
	description string
	inputs      []manifestField
	outputs     []manifestField
}

// NewManifestBuilder creates a new empty manifest builder.
func NewManifestBuilder() *ManifestBuilder {
	return &ManifestBuilder{}
}

// Description sets the block description.
func (b *ManifestBuilder) Description(desc string) *ManifestBuilder {
	b.description = desc
	return b
}

// Build returns the manifest as a map suitable for YAML serialization.
func (b *ManifestBuilder) Build() map[string]any {
	manifest := make(map[string]any)

	if b.description != "" {
		manifest["description"] = b.description
	}

	inputs := make(map[string]any)
	for _, f := range b.inputs {
		inputs[f.name] = manifestInfoToMap(f.info)
	}
	manifest["inputs"] = inputs

	outputs := make(map[string]any)
	for _, f := range b.outputs {
		outputs[f.name] = manifestInfoToMap(f.info)
	}
	manifest["outputs"] = outputs

	return manifest
}

func manifestInfoToMap(info ManifestInfo) map[string]any {
	m := map[string]any{"type": info.TypeName}
	if info.Format != "" {
		m["format"] = info.Format
	}
	if info.ItemType != "" {
		m["item_type"] = info.ItemType
	}
	return m
}

// ManifestInput declares an input on the builder using a SpadeType's manifest metadata.
func ManifestInput[T SpadeType](b *ManifestBuilder, name string) *ManifestBuilder {
	var zero T
	b.inputs = append(b.inputs, manifestField{name: name, info: zero.ManifestEntry()})
	return b
}

// ManifestOutput declares an output on the builder using a SpadeType's manifest metadata.
func ManifestOutput[T SpadeType](b *ManifestBuilder, name string) *ManifestBuilder {
	var zero T
	b.outputs = append(b.outputs, manifestField{name: name, info: zero.ManifestEntry()})
	return b
}
