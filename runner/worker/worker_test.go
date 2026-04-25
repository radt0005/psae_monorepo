package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"core"
	spade "spade_runner"

	"github.com/google/uuid"
)

// fakeExecutor is a test double that stands in for core.Execute.
// It records the arguments it was called with and returns a pre-baked
// BlockInvocationResult so tests can drive every code path through
// Worker.Run without needing the isolate sandbox.
type fakeExecutor struct {
	called  bool
	gotInv  core.BlockInvocation
	result  core.BlockInvocationResult
	err     error
	onCall  func(workDir string) // allow the test to stage outputs
	workDir string
}

func (f *fakeExecutor) Execute(
	block core.BlockInvocation,
	pipelineDir string,
	manifest core.BlockManifest,
	entry core.BlockRegistryEntry,
	reg *core.BlockRegistry,
) (core.BlockInvocationResult, error) {
	f.called = true
	f.gotInv = block
	f.workDir = filepath.Join(pipelineDir, block.InvocationID())
	if f.onCall != nil {
		f.onCall(f.workDir)
	}
	// The result needs the LogsPath derived from the work dir, matching
	// what real core.Execute would populate.
	res := f.result
	res.LogsPath = filepath.Join(f.workDir, "logs")
	return res, f.err
}

// setupRegistry opens an isolated SQLite registry in a temp dir.
func setupRegistry(t *testing.T) (*core.BlockRegistry, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "registry.db")
	reg, err := core.OpenRegistry(dbPath)
	if err != nil {
		t.Fatalf("opening registry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg, dir
}

// installFakeBlock registers a block in the registry whose manifest
// lives at <installedPath>/blocks/<blockName>.yaml.
func installFakeBlock(t *testing.T, reg *core.BlockRegistry, installedRoot string, manifest core.BlockManifest, blockName string) core.BlockRegistryEntry {
	t.Helper()
	blocksDir := filepath.Join(installedRoot, "blocks")
	if err := os.MkdirAll(blocksDir, 0755); err != nil {
		t.Fatalf("mkdir blocks: %v", err)
	}
	// Write the manifest YAML.  Use core's loader to ensure the
	// same round-trip path the worker uses.
	yamlData := []byte("id: " + manifest.ID + "\nversion: " + manifest.Version + "\nkind: " + string(manifest.Kind) + "\ninputs: {}\noutputs: {}\n")
	if err := os.WriteFile(filepath.Join(blocksDir, blockName+".yaml"), yamlData, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Compute a content hash over the install tree so VerifyBlock passes.
	hash, err := core.ComputeContentHash(installedRoot)
	if err != nil {
		t.Fatalf("content hash: %v", err)
	}
	entry := core.BlockRegistryEntry{
		CollectionName:    "fakepkg",
		CollectionVersion: manifest.Version,
		BlockName:         blockName,
		BlockID:           manifest.ID,
		Language:          string(core.CollectionLanguageGo),
		Entrypoint:        blockName,
		InstalledPath:     installedRoot,
		ContentHash:       hash,
		Kind:              string(manifest.Kind),
	}
	if err := reg.RegisterBlock(entry); err != nil {
		t.Fatalf("register block: %v", err)
	}
	return entry
}

// newStandardJob builds a Job for a single-block pipeline.
// The WorkerAssignment.BlockName is the fully-qualified manifest ID
// (e.g. "pkg.hello"), matching how the scheduler refers to block types.
func newStandardJob(manifest core.BlockManifest, args map[string]any) (spade.Job, uuid.UUID) {
	blockID := uuid.New()
	pipeID := uuid.New()
	return spade.Job{
		Assignment: core.WorkerAssignment{
			InvocationID: blockID.String(),
			BlockName:    manifest.ID,
			PipelineID:   pipeID,
			Args:         args,
		},
		Pipeline: core.Pipeline{
			Id: pipeID,
			Blocks: []core.PipelineBlock{
				{Id: blockID, Name: manifest.ID, Args: args},
			},
		},
		Manifests: map[string]core.BlockManifest{manifest.ID: manifest},
	}, blockID
}

func TestRun_HappyPath(t *testing.T) {
	reg, root := setupRegistry(t)
	installed := filepath.Join(root, "install")
	manifest := core.BlockManifest{
		ID: "pkg.hello", Version: "1.0.0", Kind: core.BlockKindStandard,
		Inputs: map[string]core.InputDeclaration{},
		Outputs: map[string]core.OutputDeclaration{
			"message": {Type: "file"},
		},
	}
	installFakeBlock(t, reg, installed, manifest, "hello")

	job, _ := newStandardJob(manifest, map[string]any{"greeting": "hi"})
	job.Assignment.WorkDir = filepath.Join(root, "work")

	fake := &fakeExecutor{
		result: core.BlockInvocationResult{
			Status:   core.ExecutionStatusComplete,
			ExitCode: 0,
		},
		onCall: func(workDir string) {
			// Stage one output file so CollectOutputs returns a hash.
			outDir := filepath.Join(workDir, "outputs", "message")
			_ = os.MkdirAll(outDir, 0755)
			_ = os.WriteFile(filepath.Join(outDir, "data.txt"), []byte("hi"), 0644)
		},
	}
	w := New(reg, filepath.Join(root, "work"), WithExecutor(fake))
	res, err := w.Run(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected infra error: %v", err)
	}
	if res.Status != core.ExecutionStatusComplete {
		t.Fatalf("expected Complete, got %s: %s", res.Status, res.Error)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", res.ExitCode)
	}
	if _, ok := res.OutputHashes["message"]; !ok {
		t.Errorf("expected output hash for 'message', got %+v", res.OutputHashes)
	}
	if res.LogsPath == "" {
		t.Errorf("expected LogsPath to be set")
	}
	if !fake.called {
		t.Errorf("fake executor was not called")
	}
}

func TestRun_BlockNotInstalled(t *testing.T) {
	reg, root := setupRegistry(t)
	manifest := core.BlockManifest{ID: "pkg.missing", Version: "1.0.0"}
	job, _ := newStandardJob(manifest, nil)
	job.Assignment.WorkDir = filepath.Join(root, "work")

	fake := &fakeExecutor{}
	w := New(reg, filepath.Join(root, "work"), WithExecutor(fake))
	res, err := w.Run(context.Background(), job)
	if err != nil {
		t.Fatalf("expected block failure to return nil err, got %v", err)
	}
	if res.Status != core.ExecutionStatusError {
		t.Fatalf("expected Error status, got %s", res.Status)
	}
	if fake.called {
		t.Error("executor should not have been called for uninstalled block")
	}
}

func TestRun_NilRegistryIsInfraFailure(t *testing.T) {
	w := New(nil, "")
	job, _ := newStandardJob(core.BlockManifest{}, nil)
	_, err := w.Run(context.Background(), job)
	if !errors.Is(err, ErrRegistryUnavailable) {
		t.Fatalf("expected ErrRegistryUnavailable, got %v", err)
	}
}

func TestRun_BlockExitsNonZero(t *testing.T) {
	reg, root := setupRegistry(t)
	installed := filepath.Join(root, "install")
	manifest := core.BlockManifest{
		ID: "pkg.broken", Version: "1.0.0", Kind: core.BlockKindStandard,
	}
	installFakeBlock(t, reg, installed, manifest, "broken")

	job, _ := newStandardJob(manifest, nil)
	job.Assignment.WorkDir = filepath.Join(root, "work")

	fake := &fakeExecutor{
		result: core.BlockInvocationResult{
			Status:   core.ExecutionStatusError,
			ExitCode: 1,
			Error:    "block exited with code 1: boom",
		},
	}
	w := New(reg, filepath.Join(root, "work"), WithExecutor(fake))
	res, err := w.Run(context.Background(), job)
	if err != nil {
		t.Fatalf("block failure should not surface as infra error: %v", err)
	}
	if res.Status != core.ExecutionStatusError {
		t.Errorf("expected Error status, got %s", res.Status)
	}
	if res.ExitCode != 1 {
		t.Errorf("expected exit 1, got %d", res.ExitCode)
	}
	if res.Error == "" {
		t.Errorf("expected Error to carry stderr tail")
	}
}

func TestRun_MapBlockAttachesExpansion(t *testing.T) {
	reg, root := setupRegistry(t)
	installed := filepath.Join(root, "install")
	manifest := core.BlockManifest{
		ID: "pkg.fan", Version: "1.0.0", Kind: core.BlockKindMap,
		Outputs: map[string]core.OutputDeclaration{
			"manifest": {Type: "expansion"},
		},
	}
	installFakeBlock(t, reg, installed, manifest, "fan")

	job, _ := newStandardJob(manifest, nil)
	job.Assignment.WorkDir = filepath.Join(root, "work")

	expansion := &core.ExpansionManifest{Items: []core.ExpansionItem{
		{Path: "inputs/source/a.tif", Key: "a"},
		{Path: "inputs/source/b.tif", Key: "b"},
	}}
	fake := &fakeExecutor{
		result: core.BlockInvocationResult{
			Status:    core.ExecutionStatusMap,
			ExitCode:  0,
			Expansion: expansion,
		},
	}
	w := New(reg, filepath.Join(root, "work"), WithExecutor(fake))
	res, err := w.Run(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != core.ExecutionStatusMap {
		t.Fatalf("expected Map status, got %s", res.Status)
	}
	if res.Expansion == nil || len(res.Expansion.Items) != 2 {
		t.Fatalf("expansion not propagated: %+v", res.Expansion)
	}
}

func TestRun_DerivesWorkDirFromWorkRoot(t *testing.T) {
	reg, root := setupRegistry(t)
	installed := filepath.Join(root, "install")
	manifest := core.BlockManifest{ID: "pkg.hello", Version: "1.0.0"}
	installFakeBlock(t, reg, installed, manifest, "hello")

	job, _ := newStandardJob(manifest, nil)
	// No explicit WorkDir — worker should derive from WorkRoot + pipeline ID.

	fake := &fakeExecutor{
		result: core.BlockInvocationResult{
			Status:   core.ExecutionStatusComplete,
			ExitCode: 0,
		},
	}
	workRoot := filepath.Join(root, "work")
	w := New(reg, workRoot, WithExecutor(fake))
	_, err := w.Run(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(workRoot, job.Assignment.PipelineID.String())
	if !isAncestor(expected, fake.workDir) {
		t.Errorf("derived work dir %q should be under %q", fake.workDir, expected)
	}
}

func TestRun_MissingDepManifestIsBlockFailure(t *testing.T) {
	reg, root := setupRegistry(t)
	installed := filepath.Join(root, "install")
	manifest := core.BlockManifest{
		ID: "pkg.sink", Version: "1.0.0", Kind: core.BlockKindStandard,
		Inputs: map[string]core.InputDeclaration{
			"in": {Type: "file"},
		},
	}
	installFakeBlock(t, reg, installed, manifest, "sink")

	depID := uuid.New()
	blockID := uuid.New()
	pipeID := uuid.New()
	job := spade.Job{
		Assignment: core.WorkerAssignment{
			InvocationID: blockID.String(),
			BlockName:    "sink",
			PipelineID:   pipeID,
			WorkDir:      filepath.Join(root, "work"),
		},
		Pipeline: core.Pipeline{
			Id: pipeID,
			Blocks: []core.PipelineBlock{
				{Id: depID, Name: "source"},
				{Id: blockID, Name: "sink", Inputs: []core.InputRef{{ID: depID}}},
			},
		},
		// Manifests does NOT include "source", so DependencyManifests fails.
		Manifests: map[string]core.BlockManifest{"sink": manifest},
	}

	fake := &fakeExecutor{}
	w := New(reg, filepath.Join(root, "work"), WithExecutor(fake))
	res, err := w.Run(context.Background(), job)
	if err != nil {
		t.Fatalf("expected block failure, got infra error: %v", err)
	}
	if res.Status != core.ExecutionStatusError {
		t.Fatalf("expected Error status, got %s", res.Status)
	}
	if fake.called {
		t.Error("executor should not have been called when deps are unresolvable")
	}
}

func TestRun_ContextAlreadyCancelledIsInfra(t *testing.T) {
	reg, _ := setupRegistry(t)
	w := New(reg, "/tmp/work")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	job, _ := newStandardJob(core.BlockManifest{}, nil)
	_, err := w.Run(ctx, job)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// isAncestor reports whether parent is an ancestor of (or equal to) child
// as filesystem paths after cleaning.
func isAncestor(parent, child string) bool {
	p := filepath.Clean(parent)
	c := filepath.Clean(child)
	rel, err := filepath.Rel(p, c)
	if err != nil {
		return false
	}
	return rel == "." || (len(rel) > 0 && rel[0] != '.')
}
