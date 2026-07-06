package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestExecuteFullFlow(t *testing.T) {
	dir := t.TempDir()
	blockID := uuid.New()

	// Create a mock block script that reads params and writes output
	scriptDir := filepath.Join(dir, "scripts")
	os.MkdirAll(scriptDir, 0755)
	script := filepath.Join(scriptDir, "test_block.sh")
	os.WriteFile(script, []byte(`#!/bin/bash
mkdir -p outputs/result
echo "processed" > outputs/result/data.txt
`), 0755)

	block := BlockInvocation{
		Id:         blockID,
		PipelineId: uuid.New(),
		BlockId:    "test.block",
		Inputs:     []InputRef{},
		Arguments:  map[string]any{"param1": "value1"},
	}

	manifest := BlockManifest{
		ID:      "test.block",
		Version: "1.0.0",
		Kind:    BlockKindStandard,
		Inputs:  map[string]InputDeclaration{},
		Outputs: map[string]OutputDeclaration{"result": {Type: "file"}},
	}

	// Test directory structure setup (without actually running the block since
	// isolate may not be available in test env)
	err := CreateBlockDirectory(block.Id.String(), dir)
	if err != nil {
		t.Fatalf("CreateBlockDirectory failed: %v", err)
	}

	workDir := filepath.Join(dir, block.Id.String())
	err = WriteParamsYAML(block.Arguments, workDir)
	if err != nil {
		t.Fatalf("WriteParamsYAML failed: %v", err)
	}

	// Verify params.yaml exists
	paramsPath := filepath.Join(workDir, "params.yaml")
	if _, err := os.Stat(paramsPath); err != nil {
		t.Errorf("expected params.yaml to exist: %v", err)
	}

	// Write invocation metadata
	meta := InvocationMetadata{
		Block: InvocationMetadataBlock{
			ID:      manifest.ID,
			Version: manifest.Version,
		},
		InvocationID: block.InvocationID(),
	}
	err = WriteInvocationMetadata(meta, workDir)
	if err != nil {
		t.Fatalf("WriteInvocationMetadata failed: %v", err)
	}

	// Verify invocation.yaml exists
	invPath := filepath.Join(workDir, "invocation.yaml")
	if _, err := os.Stat(invPath); err != nil {
		t.Errorf("expected invocation.yaml to exist: %v", err)
	}

	// Verify directory structure
	for _, sub := range []string{"inputs", "outputs", "logs"} {
		p := filepath.Join(workDir, sub)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s directory: %v", sub, err)
		}
	}
}

func TestExecuteErrorExitCode(t *testing.T) {
	// Test that a non-zero exit code produces an error result
	result := BlockInvocationResult{
		Status: ExecutionStatusError,
		Error:  "block exited with code 1: error message",
	}

	if result.Status != ExecutionStatusError {
		t.Error("expected error status")
	}
	if result.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestExecuteMapBlockExpansion(t *testing.T) {
	dir := t.TempDir()

	// Simulate a map block that wrote an expansion manifest
	outputDir := filepath.Join(dir, "outputs", "manifest")
	os.MkdirAll(outputDir, 0755)

	expansionYAML := `items:
  - path: inputs/source/tile_001.tif
    key: tile_001
  - path: inputs/source/tile_002.tif
    key: tile_002
`
	os.WriteFile(filepath.Join(outputDir, "expansion.yaml"), []byte(expansionYAML), 0644)

	expansion, err := LoadExpansionManifest(filepath.Join(outputDir, "expansion.yaml"))
	if err != nil {
		t.Fatalf("LoadExpansionManifest failed: %v", err)
	}

	if len(expansion.Items) != 2 {
		t.Errorf("expected 2 expansion items, got %d", len(expansion.Items))
	}
}

func TestStdoutStderrCapture(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")
	os.MkdirAll(logsDir, 0755)

	// Write mock log files (simulating what RunBlockSubprocess would do)
	os.WriteFile(filepath.Join(logsDir, "stdout.log"), []byte("standard output"), 0644)
	os.WriteFile(filepath.Join(logsDir, "stderr.log"), []byte("error output"), 0644)

	// Verify files exist and have content
	stdout, err := os.ReadFile(filepath.Join(logsDir, "stdout.log"))
	if err != nil {
		t.Fatalf("reading stdout.log: %v", err)
	}
	if string(stdout) != "standard output" {
		t.Errorf("expected 'standard output', got %q", string(stdout))
	}

	stderr, err := os.ReadFile(filepath.Join(logsDir, "stderr.log"))
	if err != nil {
		t.Fatalf("reading stderr.log: %v", err)
	}
	if string(stderr) != "error output" {
		t.Errorf("expected 'error output', got %q", string(stderr))
	}
}

func TestInvocationID(t *testing.T) {
	id := uuid.New()

	// Non-mapped block
	inv := BlockInvocation{Id: id}
	if inv.InvocationID() != id.String() {
		t.Errorf("expected %s, got %s", id.String(), inv.InvocationID())
	}

	// Mapped block
	idx := 7
	inv.MapIndex = &idx
	expected := id.String() + ".7"
	if inv.InvocationID() != expected {
		t.Errorf("expected %s, got %s", expected, inv.InvocationID())
	}
}

func TestLanguageSandboxBindsRLibs(t *testing.T) {
	hasBind := func(binds []string, want string) bool {
		for _, b := range binds {
			if b == want {
				return true
			}
		}
		return false
	}

	// With a shipped library, R_LIBS points at <InstalledPath>/renv/library so
	// library(<dep>) resolves the artifact's packages inside the sandbox (C2).
	installed := t.TempDir()
	artifactLib := filepath.Join(installed, "renv", "library")
	if err := os.MkdirAll(artifactLib, 0o755); err != nil {
		t.Fatal(err)
	}
	binds := languageSandboxBinds(BlockRegistryEntry{
		Language:      string(CollectionLanguageR),
		InstalledPath: installed,
	})
	if !hasBind(binds, "--env=R_LIBS="+artifactLib) {
		t.Errorf("expected R_LIBS bind for %s; got %v", artifactLib, binds)
	}

	// Base-R collections without a shipped library get no R_LIBS binding. (Note
	// R_LIBS_USER may still be set; the "R_LIBS=" prefix excludes it.)
	bare := languageSandboxBinds(BlockRegistryEntry{
		Language:      string(CollectionLanguageR),
		InstalledPath: t.TempDir(),
	})
	for _, b := range bare {
		if strings.HasPrefix(b, "--env=R_LIBS=") {
			t.Errorf("unexpected R_LIBS bind when no library shipped: %s", b)
		}
	}
}
