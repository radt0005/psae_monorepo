package cmd

import (
	"core"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var (
	noUI        bool
	keepWorkDir bool
)

var runCmd = &cobra.Command{
	Use:   "run <pipeline.yaml>",
	Short: "Run a pipeline locally",
	Long:  `Runs the specified pipeline locally using the single-instance scheduler.`,
	Args:  cobra.ExactArgs(1),
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

	// Phase 7.1: Load and validate pipeline
	pipeline, err := core.LoadPipeline(pipelinePath)
	if err != nil {
		return fmt.Errorf("loading pipeline: %w", err)
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
		if invocation.MapIndex != nil {
			blockLabel = fmt.Sprintf("%s[%d]", invocation.BlockId, *invocation.MapIndex)
		}

		// Cache check.  The cache key includes MapIndex so that each mapped
		// invocation (which consumes a different expansion item) gets its
		// own cache entry rather than sharing one with its siblings.
		inputHashes := buildInputHashes(invocation, outputHashesByInvocation, pipelineBlockByID)
		cacheArgs := invocation.Arguments
		if invocation.MapIndex != nil {
			cacheArgs = make(map[string]any, len(invocation.Arguments)+1)
			for k, v := range invocation.Arguments {
				cacheArgs[k] = v
			}
			cacheArgs["__map_index__"] = *invocation.MapIndex
		}
		cacheKey, cacheErr := core.ComputeCacheKey(manifest.ID, manifest.Version, inputHashes, cacheArgs)
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
			if resolveErr == nil {
				workDir := filepath.Join(pipelineDir, invID)
				core.SetupInputSymlinks(workDir, resolvedInputs, pipelineDir, invocation, manifest, depManifests)
			}
		}

		// Execute
		fmt.Printf("  [%d/%d] %s running...\n", completedCount+1, totalBlocks, blockLabel)
		result, err := core.Execute(invocation, pipelineDir, manifest, regEntry, registry)
		if err != nil {
			return fmt.Errorf("executing block %s: %w", blockLabel, err)
		}

		// Record output hashes on success (for Complete and Map results).
		// Map blocks return ExecutionStatusMap but still produce outputs —
		// their expansion manifest must be hashed so downstream mapped
		// blocks get distinct cache keys across pipelines.
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
			fmt.Fprintf(os.Stderr, "Block %s failed: %s\n", blockLabel, result.Error)
			os.Exit(1)
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
		} else if invocation.MapIndex != nil {
			peer := fmt.Sprintf("%s.%d", depKey, *invocation.MapIndex)
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

