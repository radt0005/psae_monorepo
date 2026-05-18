package api

import (
	"bytes"
	"io"
)

// readAll consumes an io.Reader fully and returns its bytes.  Wraps
// io.ReadAll just so the import surface in server.go stays minimal.
func readAll(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
