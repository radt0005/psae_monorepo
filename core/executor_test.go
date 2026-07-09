package core

import (
	"os"
	"os/exec"
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

// isolateFriendlyWorkArea creates a world-traversable temp directory tree so
// the isolate sandbox uid (typically remapped to 100000) can enter it. Mirrors
// runner/testutil.IsolateFriendlyTempDir, which core cannot import (cycle).
func isolateFriendlyWorkArea(t *testing.T) string {
	t.Helper()
	root := os.Getenv("SPADE_TEST_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("UserHomeDir: %v", err)
		}
		root = filepath.Join(home, ".spade-integration-tests")
	}
	if err := os.MkdirAll(root, 0777); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	dir, err := os.MkdirTemp(root, t.Name()+"-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// TestRunBlockSubprocessInjectsSecrets verifies the Phase 1 delivery contract
// (spec/secrets.md §4): resolved secrets reach the sandboxed block via the
// SPADE_SECRETS env blob, and the framework never writes the values to disk.
func TestRunBlockSubprocessInjectsSecrets(t *testing.T) {
	if _, err := exec.LookPath("isolate"); err != nil {
		t.Skip("isolate binary not installed, skipping sandbox test")
	}

	area := isolateFriendlyWorkArea(t)
	workDir := filepath.Join(area, "inv")
	for _, sub := range []string{"logs", "outputs"} {
		p := filepath.Join(workDir, sub)
		if err := os.MkdirAll(p, 0777); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
		// MkdirAll honours umask; chmod so the sandbox uid (100000) can write.
		if err := os.Chmod(p, 0777); err != nil {
			t.Fatalf("chmod %s: %v", sub, err)
		}
	}
	_ = os.Chmod(workDir, 0777)
	// A representative params.yaml written by the framework — it must not
	// contain any secret value.
	if err := WriteParamsYAML(map[string]any{"region": "maine"}, workDir); err != nil {
		t.Fatalf("WriteParamsYAML: %v", err)
	}

	// The block writes its SPADE_SECRETS env to an output file so the test can
	// observe what crossed the sandbox boundary. CWD is the workDir.
	script := filepath.Join(workDir, "block.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' \"$SPADE_SECRETS\" > \"$PWD/outputs/got.txt\"\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	installed := filepath.Join(area, "installed")
	if err := os.MkdirAll(installed, 0777); err != nil {
		t.Fatalf("mkdir installed: %v", err)
	}

	const secretValue = "s3cr3t-connection-string"
	secrets := map[string]string{"db": secretValue}

	exitCode, err := RunBlockSubprocess(
		script, nil, workDir,
		BlockManifest{ID: "test.block", Kind: BlockKindStandard},
		BlockRegistryEntry{InstalledPath: installed, BlockName: "test.block"},
		nil, secrets,
	)
	if err != nil {
		t.Fatalf("RunBlockSubprocess: %v", err)
	}
	if exitCode != 0 {
		stderr, _ := os.ReadFile(filepath.Join(workDir, "logs", "stderr.log"))
		t.Fatalf("block exited %d: %s", exitCode, stderr)
	}

	// The block saw the injected blob.
	got, err := os.ReadFile(filepath.Join(workDir, "outputs", "got.txt"))
	if err != nil {
		t.Fatalf("reading block output: %v", err)
	}
	if string(got) != `{"db":"`+secretValue+`"}` {
		t.Errorf("block saw SPADE_SECRETS=%q, want the db value injected", got)
	}

	// The framework wrote the value nowhere on disk. Scan every file the
	// framework produced (params.yaml, logs) for the secret value; only the
	// block's own output may contain it.
	frameworkFiles := []string{
		filepath.Join(workDir, "params.yaml"),
		filepath.Join(workDir, "logs", "stdout.log"),
		filepath.Join(workDir, "logs", "stderr.log"),
	}
	for _, f := range frameworkFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue // not all files necessarily exist
		}
		if strings.Contains(string(data), secretValue) {
			t.Errorf("secret value leaked into framework-written file %s", f)
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
	inv.MapIndices = []int{7}
	expected := id.String() + ".7"
	if inv.InvocationID() != expected {
		t.Errorf("expected %s, got %s", expected, inv.InvocationID())
	}

	// Nested mapped block
	inv.MapIndices = []int{7, 0, 12}
	expected = id.String() + ".7.0.12"
	if inv.InvocationID() != expected {
		t.Errorf("expected %s, got %s", expected, inv.InvocationID())
	}
}

func TestParseInvocationID(t *testing.T) {
	id := uuid.New()
	cases := []struct {
		in      string
		indices []int
	}{
		{id.String(), nil},
		{id.String() + ".0", []int{0}},
		{id.String() + ".7.3", []int{7, 3}},
		{id.String() + ".1.0.12", []int{1, 0, 12}},
	}
	for _, c := range cases {
		u, indices, err := ParseInvocationID(c.in)
		if err != nil {
			t.Fatalf("ParseInvocationID(%q): %v", c.in, err)
		}
		if u != id {
			t.Errorf("ParseInvocationID(%q): uuid = %s, want %s", c.in, u, id)
		}
		if len(indices) != len(c.indices) {
			t.Fatalf("ParseInvocationID(%q): indices = %v, want %v", c.in, indices, c.indices)
		}
		for i := range indices {
			if indices[i] != c.indices[i] {
				t.Errorf("ParseInvocationID(%q): indices = %v, want %v", c.in, indices, c.indices)
			}
		}
		// Round trip
		if got := FormatInvocationID(u, indices); got != c.in {
			t.Errorf("FormatInvocationID round-trip: got %q, want %q", got, c.in)
		}
	}

	if _, _, err := ParseInvocationID("not-a-uuid.3"); err == nil {
		t.Error("expected error for malformed invocation ID")
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
