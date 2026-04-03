package spade

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// InputKind distinguishes single-file from multi-file input entries.
type InputKind int

const (
	InputSingle   InputKind = iota
	InputMultiple
)

// InputEntry represents a scanned input from the inputs/ directory.
type InputEntry struct {
	Kind  InputKind
	Path  string   // populated for InputSingle
	Paths []string // populated for InputMultiple
}

// LoadParams reads and parses params.yaml from the given base directory.
// Returns an empty map if the file does not exist or is empty.
func LoadParams(base string) (map[string]any, error) {
	path := filepath.Join(base, "params.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var params map[string]any
	if err := yaml.Unmarshal(data, &params); err != nil {
		return nil, err
	}
	if params == nil {
		return map[string]any{}, nil
	}
	return params, nil
}

// ScanInputs reads the inputs/ directory at the given base path and returns
// a map of input name to InputEntry.
func ScanInputs(base string) (map[string]InputEntry, error) {
	inputsDir := filepath.Join(base, "inputs")
	subdirs, err := os.ReadDir(inputsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]InputEntry{}, nil
		}
		return nil, err
	}

	// Sort subdirectories by name
	sort.Slice(subdirs, func(i, j int) bool {
		return subdirs[i].Name() < subdirs[j].Name()
	})

	result := make(map[string]InputEntry)
	for _, subdir := range subdirs {
		if !subdir.IsDir() {
			continue
		}
		name := subdir.Name()
		subdirPath := filepath.Join(inputsDir, name)

		entries, err := os.ReadDir(subdirPath)
		if err != nil {
			return nil, err
		}

		var files []string
		for _, entry := range entries {
			if !entry.IsDir() {
				files = append(files, filepath.Join(subdirPath, entry.Name()))
			}
		}
		sort.Strings(files)

		if len(files) == 0 {
			return nil, &ErrEmptyInputDir{Name: name}
		}

		if len(files) == 1 {
			result[name] = InputEntry{Kind: InputSingle, Path: files[0]}
		} else {
			result[name] = InputEntry{Kind: InputMultiple, Paths: files}
		}
	}
	return result, nil
}

// Args holds merged parameters and inputs for handler consumption.
type Args struct {
	params map[string]any
	inputs map[string]InputEntry
}

// HasInput checks whether an input with the given name exists.
func (a *Args) HasInput(name string) bool {
	_, ok := a.inputs[name]
	return ok
}

// HasParam checks whether a parameter with the given name exists.
func (a *Args) HasParam(name string) bool {
	_, ok := a.params[name]
	return ok
}

// BuildArgs constructs an Args struct from the given base path by loading
// params.yaml and scanning the inputs/ directory.
func BuildArgs(base string) (*Args, error) {
	params, err := LoadParams(base)
	if err != nil {
		return nil, err
	}
	inputs, err := ScanInputs(base)
	if err != nil {
		return nil, err
	}
	return &Args{params: params, inputs: inputs}, nil
}

// Input retrieves a typed file/directory input by name from Args.
// T must be a pointer to a type implementing FromInput.
func Input[T interface {
	*E
	FromInput
}, E any](args *Args, name string) (E, error) {
	var zero E
	entry, ok := args.inputs[name]
	if !ok {
		return zero, &ErrInputNotFound{Name: name}
	}
	val := T(&zero)
	switch entry.Kind {
	case InputSingle:
		if err := val.FromSingleFile(entry.Path); err != nil {
			return zero, err
		}
	case InputMultiple:
		if err := val.FromMultipleFiles(entry.Paths); err != nil {
			return zero, err
		}
	}
	return zero, nil
}

// Param retrieves a typed scalar parameter by name from Args.
func Param[T any](args *Args, name string) (T, error) {
	var zero T
	raw, ok := args.params[name]
	if !ok {
		return zero, &ErrParamNotFound{Name: name}
	}

	// Try direct type assertion first
	if v, ok := raw.(T); ok {
		return v, nil
	}

	// Handle numeric conversions: yaml.v3 parses integers as int, floats as float64
	switch any(zero).(type) {
	case float64:
		switch v := raw.(type) {
		case int:
			return any(float64(v)).(T), nil
		case int64:
			return any(float64(v)).(T), nil
		}
	case int:
		switch v := raw.(type) {
		case float64:
			return any(int(v)).(T), nil
		case int64:
			return any(int(v)).(T), nil
		}
	case int64:
		switch v := raw.(type) {
		case int:
			return any(int64(v)).(T), nil
		case float64:
			return any(int64(v)).(T), nil
		}
	}

	// Fallback: marshal to YAML then unmarshal into T
	data, err := yaml.Marshal(raw)
	if err != nil {
		return zero, fmt.Errorf("parameter '%s': cannot convert value: %w", name, err)
	}
	var result T
	if err := yaml.Unmarshal(data, &result); err != nil {
		return zero, fmt.Errorf("parameter '%s': cannot convert to target type: %w", name, err)
	}
	return result, nil
}
