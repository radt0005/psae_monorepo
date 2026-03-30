package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Execute runs a block invocation through the full lifecycle:
// verify, set up directory, write params, set up inputs, run subprocess, collect outputs.
func Execute(block BlockInvocation, pipelineDir string, manifest BlockManifest, registryEntry BlockRegistryEntry, registry *BlockRegistry) (BlockInvocationResult, error) {
	result := BlockInvocationResult{
		Id:         block.Id,
		PipelineId: block.PipelineId,
	}

	// Verify block integrity
	if registry != nil {
		if err := registry.VerifyBlock(registryEntry); err != nil {
			result.Status = ExecutionStatusError
			result.Error = fmt.Sprintf("block integrity check failed: %v", err)
			return result, err
		}
	}

	// Create directory structure
	if err := CreateBlockDirectory(block.Id, pipelineDir); err != nil {
		result.Status = ExecutionStatusError
		result.Error = fmt.Sprintf("creating block directory: %v", err)
		return result, err
	}

	workDir := filepath.Join(pipelineDir, block.Id.String())

	// Write params.yaml
	if err := WriteParamsYAML(block.Arguments, workDir); err != nil {
		result.Status = ExecutionStatusError
		result.Error = fmt.Sprintf("writing params: %v", err)
		return result, err
	}

	// Write invocation.yaml
	meta := InvocationMetadata{
		Block: InvocationMetadataBlock{
			ID:      manifest.ID,
			Version: manifest.Version,
		},
		InvocationID: block.InvocationID(),
	}
	if err := WriteInvocationMetadata(meta, workDir); err != nil {
		result.Status = ExecutionStatusError
		result.Error = fmt.Sprintf("writing invocation metadata: %v", err)
		return result, err
	}

	// Resolve entrypoint
	execPath, args, err := ResolveEntrypoint(registryEntry)
	if err != nil {
		result.Status = ExecutionStatusError
		result.Error = fmt.Sprintf("resolving entrypoint: %v", err)
		return result, err
	}

	// Run the subprocess with isolate
	exitCode, err := RunBlockSubprocess(execPath, args, workDir, manifest)
	if err != nil {
		result.Status = ExecutionStatusError
		result.Error = fmt.Sprintf("subprocess execution failed: %v", err)
		return result, err
	}

	if exitCode != 0 {
		// Read stderr for error info
		stderrPath := filepath.Join(workDir, "logs", "stderr.log")
		stderrData, _ := os.ReadFile(stderrPath)
		result.Status = ExecutionStatusError
		result.Error = fmt.Sprintf("block exited with code %d: %s", exitCode, string(stderrData))
		return result, nil
	}

	// If map block, read expansion manifest
	if manifest.Kind == BlockKindMap {
		for outputName, outputDecl := range manifest.Outputs {
			if outputDecl.Type == "expansion" {
				expansionPath := filepath.Join(workDir, "outputs", outputName, "expansion.yaml")
				expansion, err := LoadExpansionManifest(expansionPath)
				if err != nil {
					result.Status = ExecutionStatusError
					result.Error = fmt.Sprintf("reading expansion manifest: %v", err)
					return result, nil
				}
				result.Expansion = &expansion
				result.Status = ExecutionStatusMap
				return result, nil
			}
		}
	}

	// Collect outputs
	outputHashes, err := CollectOutputs(workDir)
	if err != nil {
		result.Status = ExecutionStatusError
		result.Error = fmt.Sprintf("collecting outputs: %v", err)
		return result, nil
	}
	result.Outputs = make([]string, 0, len(outputHashes))
	for name := range outputHashes {
		result.Outputs = append(result.Outputs, name)
	}

	result.Status = ExecutionStatusComplete
	return result, nil
}

// RunBlockSubprocess executes the block as a subprocess using isolate for sandboxing.
func RunBlockSubprocess(execPath string, args []string, workDir string, manifest BlockManifest) (int, error) {
	// Build isolate command
	isolateArgs := []string{
		"--dir=" + workDir,
		"--mem=512000", // 512MB default
		"--time=3600",  // 1 hour default
	}

	if !manifest.Network {
		isolateArgs = append(isolateArgs, "--no-network")
	}

	// Add the actual command
	isolateArgs = append(isolateArgs, "--run", "--", execPath)
	isolateArgs = append(isolateArgs, args...)

	cmd := exec.Command("isolate", isolateArgs...)
	cmd.Dir = workDir

	// Capture stdout and stderr to log files
	stdoutPath := filepath.Join(workDir, "logs", "stdout.log")
	stderrPath := filepath.Join(workDir, "logs", "stderr.log")

	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return -1, fmt.Errorf("creating stdout log: %w", err)
	}
	defer stdoutFile.Close()

	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		return -1, fmt.Errorf("creating stderr log: %w", err)
	}
	defer stderrFile.Close()

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}

	return 0, nil
}
