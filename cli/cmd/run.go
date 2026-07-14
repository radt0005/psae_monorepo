package cmd

import (
	"core"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"spade/internal/secretstore"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var (
	noUI        bool
	keepWorkDir bool
)

// errNoCache marks invocations that bypass the result cache entirely
// (map blocks; see the cache-check comment in runPipeline).
var errNoCache = errors.New("caching disabled for this block")

var runCmd = &cobra.Command{
	Use:   "run <pipeline.yaml>",
	Short: "Run a pipeline locally",
	Long:  `Runs the specified pipeline locally using the single-instance scheduler.`,
	Args:  cobra.ExactArgs(1),
	// Runtime failures (block errors, missing inputs) are not usage
	// errors; suppress the usage dump cobra prints for returned errors.
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPipeline(args[0])
	},
}

func init() {
	runCmd.Flags().BoolVar(&noUI, "no-ui", false, "Disable BubbleTea interface, use simple output")
	runCmd.Flags().BoolVar(&keepWorkDir, "keep-work-dir", false, "Preserve the pipeline working directory after execution")
	rootCmd.AddCommand(runCmd)
}

func runPipeline(pipelinePath string) error {
	startTime := time.Now()

	// Phase 7.1: Load (resolving any short codes via the sibling
	// lockfile) and validate the pipeline.  See spec/pipeline.md §6.
	pipeline, _, wroteLockfile, err := core.LoadAndResolvePipeline(pipelinePath)
	if err != nil {
		if errors.Is(err, core.ErrInvalidLockfile) {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			fmt.Fprintf(os.Stderr, "To regenerate the lockfile from scratch, delete %s.\n",
				core.LockfilePathFor(pipelinePath))
			os.Exit(1)
		}
		return fmt.Errorf("loading pipeline: %w", err)
	}
	if wroteLockfile {
		fmt.Printf("Wrote %s\n", core.LockfilePathFor(pipelinePath))
	}

	// Pipeline ID is generated at run time when omitted from source
	// (spec/pipeline.md §10).  This is per-run state, never persisted.
	if pipeline.Id == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generating pipeline id: %w", err)
		}
		pipeline.Id = id
	}
	fmt.Printf("Loaded pipeline: %s (%s)\n", pipeline.Name, pipeline.Id)

	registry, err := core.OpenRegistry(RegistryPath())
	if err != nil {
		return fmt.Errorf("opening registry: %w", err)
	}
	defer registry.Close()

	// Look up manifests and registry entries
	manifests := make(map[string]core.BlockManifest)
	registryEntries := make(map[string]core.BlockRegistryEntry)

	for _, block := range pipeline.Blocks {
		if _, exists := manifests[block.Name]; exists {
			continue
		}
		entry, err := registry.LookupBlock(block.Name, "")
		if err != nil {
			return fmt.Errorf("block type %q not found in registry: %w", block.Name, err)
		}
		manifestPath := findManifestForBlock(entry)
		manifest, err := core.LoadBlockManifest(manifestPath)
		if err != nil {
			return fmt.Errorf("loading manifest for %q: %w", block.Name, err)
		}
		manifests[block.Name] = manifest
		registryEntries[block.Name] = *entry
	}

	// Validate
	errs := core.ValidatePipeline(pipeline, manifests)
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Pipeline validation failed:\n")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		os.Exit(1)
	}

	// Phase 7.2: Working directory setup
	pipelineDir := filepath.Join(PipelinesDir(), pipeline.Id.String())
	if err := os.MkdirAll(pipelineDir, 0755); err != nil {
		return fmt.Errorf("creating pipeline directory: %w", err)
	}
	if !keepWorkDir {
		defer os.RemoveAll(pipelineDir)
	}
	cacheDir := CacheDir()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	// Phase 7.3: Initialize scheduler
	scheduler := core.NewSchedulerForPipeline(pipeline)
	scheduler.Manifests = manifests
	if err := scheduler.IdentifyMapContexts(); err != nil {
		return fmt.Errorf("identifying map contexts: %w", err)
	}

	// Build block lookup by ID
	pipelineBlockByID := make(map[uuid.UUID]core.PipelineBlock)
	for _, b := range pipeline.Blocks {
		pipelineBlockByID[b.Id] = b
	}

	// Context depths let symlink setup pick the right instance directory
	// for nested map/reduce dependencies.
	depDepths := core.DependencyDepths(pipeline, manifests)

	// Track output hashes for caching.  Keyed by InvocationID (the
	// `<uuid>.<index>` form for mapped invocations) so each mapped
	// invocation gets its own entry.
	outputHashesByInvocation := make(map[string]map[string]string)

	totalBlocks := len(pipeline.Blocks)
	completedCount := 0
	cachedCount := 0

	// Phase 7.4: Execution loop
	fmt.Printf("Executing pipeline with %d block(s)...\n", totalBlocks)

	for {
		invocation, done, err := scheduler.Next()
		if err != nil {
			return fmt.Errorf("scheduler error: %w", err)
		}
		if done {
			break
		}
		if invocation.BlockId == "" {
			// No block ready and not done - shouldn't happen in single-threaded mode
			break
		}

		manifest := manifests[invocation.BlockId]
		regEntry := registryEntries[invocation.BlockId]
		invID := invocation.InvocationID()

		blockLabel := invocation.BlockId
		if len(invocation.MapIndices) > 0 {
			blockLabel = fmt.Sprintf("%s[%s]", invocation.BlockId, core.IndexPrefix(invocation.MapIndices, len(invocation.MapIndices)))
		}

		// Cache check.  The cache key includes the map index vector so that
		// each mapped invocation (which consumes a different expansion item)
		// gets its own cache entry rather than sharing one with its siblings.
		//
		// Map blocks are excluded from the cache entirely — lookup and
		// store.  Their expansion items reference files in their own
		// inputs/ directory, which a cache restore does not recreate
		// (only outputs/ are stored), so a restored map block leaves any
		// downstream mapped invocation that misses cache with dangling
		// input symlinks.  Enumeration is cheap; always re-execute maps.
		// (Skipping lookup also neutralises map entries stored by
		// earlier CLI versions.)
		isMapBlock := manifest.Kind == core.BlockKindMap
		var cacheKey string
		cacheErr := errNoCache
		if !isMapBlock {
			inputHashes := buildInputHashes(invocation, outputHashesByInvocation, pipelineBlockByID)
			cacheArgs := invocation.Arguments
			if len(invocation.MapIndices) > 0 {
				cacheArgs = make(map[string]any, len(invocation.Arguments)+1)
				for k, v := range invocation.Arguments {
					cacheArgs[k] = v
				}
				cacheArgs["__map_indices__"] = core.IndexPrefix(invocation.MapIndices, len(invocation.MapIndices))
			}
			cacheKey, cacheErr = core.ComputeCacheKey(manifest.ID, manifest.Version, inputHashes, cacheArgs)
		}
		if cacheErr == nil {
			if _, found := core.CacheLookup(cacheKey, cacheDir); found {
				// Restore from cache
				workDir := filepath.Join(pipelineDir, invID)
				if err := os.MkdirAll(filepath.Join(workDir, "outputs"), 0755); err == nil {
					if err := core.CacheRestore(cacheKey, filepath.Join(workDir, "outputs"), cacheDir); err == nil {
						outputHashes, _ := core.CollectOutputs(workDir)
						outputHashesByInvocation[invID] = outputHashes
						outputs := make([]string, 0, len(outputHashes))
						for name := range outputHashes {
							outputs = append(outputs, name)
						}

						result := core.BlockInvocationResult{
							Id:         invocation.Id,
							PipelineId: pipeline.Id,
							MapIndices: invocation.MapIndices,
							Status:     core.ExecutionStatusComplete,
							Outputs:    outputs,
						}
						scheduler.Update(result)
						completedCount++
						cachedCount++
						fmt.Printf("  [%d/%d] %s (cached)\n", completedCount, totalBlocks, blockLabel)
						continue
					}
				}
			}
		}

		// Set up inputs
		pipelineBlock, ok := pipelineBlockByID[invocation.Id]
		if ok {
			depManifests := make(map[uuid.UUID]core.BlockManifest)
			for _, input := range pipelineBlock.Inputs {
				var depID uuid.UUID
				if input.Block != nil {
					depID = *input.Block
				} else {
					depID = input.ID
				}
				if depID == uuid.Nil {
					continue
				}
				if depBlock, ok := pipelineBlockByID[depID]; ok {
					if m, ok := manifests[depBlock.Name]; ok {
						depManifests[depID] = m
					}
				}
			}

			resolvedInputs, resolveErr := core.ResolveInputs(pipelineBlock, depManifests, manifest)
			if resolveErr != nil {
				return fmt.Errorf("resolving inputs for block %s: %w", blockLabel, resolveErr)
			}
			// Reset (wipe + recreate) the invocation directory before
			// input setup: with --keep-work-dir a previous run's directory
			// survives, and stale input symlinks would fail re-setup with
			// EEXIST while stale partial outputs would be collected as
			// this run's.  Cache-restored blocks never reach this path.
			if err := core.ResetBlockDirectory(invID, pipelineDir); err != nil {
				return fmt.Errorf("resetting work directory for block %s: %w", blockLabel, err)
			}
			workDir := filepath.Join(pipelineDir, invID)
			if err := core.SetupInputSymlinks(workDir, resolvedInputs, pipelineDir, invocation, manifest, depManifests, depDepths); err != nil {
				return fmt.Errorf("setting up inputs for block %s: %w", blockLabel, err)
			}
		}

		// Resolve any secrets the block declared from the local OS keychain.
		secrets, secErr := resolveBlockSecrets(pipelineBlock)
		if secErr != nil {
			return fmt.Errorf("resolving secrets for block %s: %w", blockLabel, secErr)
		}

		// Execute
		fmt.Printf("  [%d/%d] %s running...\n", completedCount+1, totalBlocks, blockLabel)
		result, err := core.Execute(invocation, pipelineDir, manifest, regEntry, registry, secrets)
		if err != nil {
			return fmt.Errorf("executing block %s: %w", blockLabel, err)
		}

		// Record output hashes on success (for Complete and Map results).
		// Map blocks return ExecutionStatusMap but still produce outputs —
		// their expansion manifest must be hashed so downstream mapped
		// blocks get distinct cache keys across pipelines.  Only Complete
		// results are stored in the cache; map blocks always re-execute
		// (see the cache-check comment above).
		if result.Status == core.ExecutionStatusComplete || result.Status == core.ExecutionStatusMap {
			workDir := filepath.Join(pipelineDir, invID)
			outputHashes, _ := core.CollectOutputs(workDir)
			outputHashesByInvocation[invID] = outputHashes

			if result.Status == core.ExecutionStatusComplete && cacheErr == nil {
				core.CacheStore(cacheKey, filepath.Join(workDir, "outputs"), cacheDir)
			}
		}

		// Update scheduler
		scheduler.Update(result)

		if result.Status == core.ExecutionStatusError {
			// Return (never os.Exit) so the deferred pipelineDir cleanup
			// runs: a stale work dir from a failed run breaks later runs —
			// sandbox-owned files trip permission checks, and leftover
			// mapped sibling dirs could be gathered by a reduce whose
			// expansion has since shrunk.  The block's stderr is embedded
			// in result.Error; rerun with --keep-work-dir for full logs.
			return fmt.Errorf("block %s failed: %s", blockLabel, result.Error)
		}

		completedCount++
		fmt.Printf("  [%d/%d] %s complete\n", completedCount, totalBlocks, blockLabel)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\nPipeline complete: %d block(s) executed", completedCount)
	if cachedCount > 0 {
		fmt.Printf(" (%d cached)", cachedCount)
	}
	fmt.Printf(" in %s\n", elapsed.Round(time.Millisecond))
	return nil
}

// resolveBlockSecrets reads the values for a block's declared secrets from the
// local OS keychain, returning a map of the block's logical names to values for
// injection into the sandbox (see spec/secrets.md). Returns nil when the block
// declares no secrets. A referenced secret missing from the keychain is a
// clear, actionable error.
func resolveBlockSecrets(pb core.PipelineBlock) (map[string]string, error) {
	if len(pb.Secrets) == 0 {
		return nil, nil
	}
	resolved := make(map[string]string, len(pb.Secrets))
	for logical, storedName := range pb.Secrets {
		val, err := secretstore.Get(storedName)
		if err != nil {
			if errors.Is(err, secretstore.ErrNotFound) {
				return nil, fmt.Errorf("secret %q (bound to %q) is not in the local keychain; "+
					"set it with `spade secret set %s`", storedName, logical, storedName)
			}
			return nil, fmt.Errorf("reading secret %q: %w", storedName, err)
		}
		resolved[logical] = val
	}
	return resolved, nil
}

func buildInputHashes(invocation core.BlockInvocation, outputHashes map[string]map[string]string, _ map[uuid.UUID]core.PipelineBlock) map[string]string {
	result := make(map[string]string)
	for _, input := range invocation.Inputs {
		var depID uuid.UUID
		if input.Block != nil {
			depID = *input.Block
		} else {
			depID = input.ID
		}
		if depID == uuid.Nil {
			continue
		}
		depKey := depID.String()
		// Resolve the dep to one or more actual invocation IDs:
		//   - exact <uuid>           → non-mapped dep or map block
		//   - <uuid>.<currentIdx>    → peer in same map context
		//   - <uuid>.*               → gather every mapped sibling (for reduce)
		var depInvocationIDs []string
		if _, ok := outputHashes[depKey]; ok {
			depInvocationIDs = []string{depKey}
		} else if len(invocation.MapIndices) > 0 {
			peer := depKey + "." + core.IndexPrefix(invocation.MapIndices, len(invocation.MapIndices))
			if _, ok := outputHashes[peer]; ok {
				depInvocationIDs = []string{peer}
			}
		}
		if len(depInvocationIDs) == 0 {
			prefix := depKey + "."
			for id := range outputHashes {
				if strings.HasPrefix(id, prefix) {
					depInvocationIDs = append(depInvocationIDs, id)
				}
			}
			sort.Strings(depInvocationIDs)
		}
		for _, depInvID := range depInvocationIDs {
			hashes := outputHashes[depInvID]
			if input.Output != "" {
				if hash, ok := hashes[input.Output]; ok {
					key := fmt.Sprintf("%s:%s", depInvID, input.Output)
					result[key] = hash
				}
			} else {
				for name, hash := range hashes {
					key := fmt.Sprintf("%s:%s", depInvID, name)
					result[key] = hash
				}
			}
		}
	}
	return result
}

