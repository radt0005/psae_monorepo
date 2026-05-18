package engine

import (
	"core"

	"gopkg.in/yaml.v3"
)

// yamlMarshalPipeline serializes a Pipeline to YAML.  Tiny helper kept
// here so tests do not need to import gopkg.in/yaml.v3 directly.
func yamlMarshalPipeline(p core.Pipeline) ([]byte, error) {
	return yaml.Marshal(&p)
}
