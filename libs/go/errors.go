package spade

import "fmt"

// ErrInputNotFound is returned when Args.Input references a name not present in inputs/.
type ErrInputNotFound struct {
	Name string
}

func (e *ErrInputNotFound) Error() string {
	return fmt.Sprintf("input not found: '%s'", e.Name)
}

// ErrParamNotFound is returned when Args.Param references a name not present in params.yaml.
type ErrParamNotFound struct {
	Name string
}

func (e *ErrParamNotFound) Error() string {
	return fmt.Sprintf("parameter not found: '%s'", e.Name)
}

// ErrEmptyInputDir is returned when an input subdirectory exists but contains no files.
type ErrEmptyInputDir struct {
	Name string
}

func (e *ErrEmptyInputDir) Error() string {
	return fmt.Sprintf("input directory '%s' is empty", e.Name)
}

// ErrTypeMismatch is returned when an input value cannot be converted to the requested type.
type ErrTypeMismatch struct {
	Name     string
	Expected string
	Found    string
}

func (e *ErrTypeMismatch) Error() string {
	return fmt.Sprintf("type mismatch for '%s': expected %s, found %s", e.Name, e.Expected, e.Found)
}
