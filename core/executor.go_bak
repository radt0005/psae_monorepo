package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// isolate ships with a fixed number of pre-created box slots (0 through
// the configured maximum — 999 by default on Ubuntu).  To run block
// subprocesses concurrently we allocate a unique box ID per invocation
// and release it when the subprocess finishes.
var (
	isolateBoxMu   sync.Mutex
	isolateBoxUsed = map[int]bool{}
	isolateBoxNext = 0
)

const isolateBoxMax = 999

func allocateIsolateBoxID() int {
	isolateBoxMu.Lock()
	defer isolateBoxMu.Unlock()
	for i := 0; i < isolateBoxMax; i++ {
		id := (isolateBoxNext + i) % isolateBoxMax
		if !isolateBoxUsed[id] {
			isolateBoxUsed[id] = true
			isolateBoxNext = (id + 1) % isolateBoxMax
			return id
		}
	}
	// Pool exhausted — fall back to 0 and let isolate surface the
	// error at --init time.  In practice, 1000 concurrent sandboxes
	// would saturate every reasonable host long before this limit.
	return 0
}

func releaseIsolateBoxID(id int) {
	isolateBoxMu.Lock()
	defer isolateBoxMu.Unlock()
	delete(isolateBoxUsed, id)
}

// Execute runs a block invocation through the full lifecycle:
// verify, set up directory, write params, set up inputs, run subprocess, collect outputs.
func Execute(block BlockInvocation, pipelineDir string, manifest BlockManifest, registryEntry BlockRegistryEntry, registry *BlockRegistry) (BlockInvocationResult, error) {
	result := BlockInvocationResult{
		Id:         block.Id,
		PipelineId: block.PipelineId,
		ExitCode:   -1,
	}

	// Verify block integrity
	if registry != nil {
		if err := registry.VerifyBlock(registryEntry); err != nil {
			result.Status = ExecutionStatusError
			result.Error = fmt.Sprintf("block integrity check failed: %v", err)
			return result, err
		}
	}

	// Create directory structure (unique per mapped invocation via
	// InvocationID, which is "<uuid>" for a normal block and
	// "<uuid>.<index>" when the block runs inside a map context).
	invID := block.InvocationID()
	if err := CreateBlockDirectory(invID, pipelineDir); err != nil {
		result.Status = ExecutionStatusError
		result.Error = fmt.Sprintf("creating block directory: %v", err)
		return result, err
	}

	workDir := filepath.Join(pipelineDir, invID)
	result.LogsPath = filepath.Join(workDir, "logs")

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

	// Collect directories referenced by file:// URIs or absolute paths in
	// args so they can be bound into the sandbox.  Also include the pipeline
	// directory so that input symlinks pointing at previous blocks' outputs
	// resolve inside the sandbox.
	argBinds := discoverArgPaths(block.Arguments)
	argBinds = append(argBinds, pipelineDir)

	// Run the subprocess with isolate
	exitCode, err := RunBlockSubprocess(execPath, args, workDir, manifest, registryEntry, argBinds)
	if err != nil {
		result.Status = ExecutionStatusError
		result.Error = fmt.Sprintf("subprocess execution failed: %v", err)
		return result, err
	}
	result.ExitCode = exitCode

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
//
// Isolate's default sandbox exposes only /bin, /dev, /lib, /lib64, /proc and /usr.
// To run a block we need to additionally expose:
//   - the pipeline work directory (read-write, since blocks write outputs)
//   - the block's installed path under ~/.spade/blocks/<collection>/<version>/
//     so the binary or script is visible
//   - any language-specific toolchain directories (uv, Rscript, bun, ...)
//
// Network access is disabled by default (isolate's new-netns); --share-net
// re-enables it for blocks that declared network: true.
//
// isolate requires `--init` to create the sandbox directory before `--run`
// and `--cleanup` to remove it after; this function handles that lifecycle
// around each invocation using a process-unique box ID to avoid
// collisions across concurrent calls.
func RunBlockSubprocess(execPath string, args []string, workDir string, manifest BlockManifest, entry BlockRegistryEntry, extraBinds []string) (int, error) {
	boxID := allocateIsolateBoxID()
	defer releaseIsolateBoxID(boxID)

	boxArg := fmt.Sprintf("--box-id=%d", boxID)

	// Create the sandbox.  If init fails, there is nothing to clean up.

	initCmd := exec.Command("isolate", boxArg, "--init")
	if out, err := initCmd.CombinedOutput(); err != nil {
		return -1, fmt.Errorf("isolate --init failed: %v: %s", err, out)
	}
	// Always clean up, even on error paths.
	defer func() {
		_ = exec.Command("isolate", boxArg, "--cleanup").Run()
	}()

	isolateArgs := []string{
		boxArg,
		"--processes=128", // allow multithreaded runtimes (tokio, uv, GDAL)
		"--dir=" + workDir + ":rw",
		"--chdir=" + workDir,
		"--dir=" + entry.InstalledPath,
		"--mem=2048000",     // 2GB address-space (per-process rlimit)
		"--time=3600",       // 1 hour CPU
		"--wall-time=3600",  // 1 hour wall
		"--fsize=2097152",   // 2GB max file size
		"--open-files=4096", // GDAL/Python open a lot of files
	}

	isolateArgs = append(isolateArgs, languageSandboxBinds(entry)...)

	// Bind directories referenced by args (file:// URIs, absolute paths)
	for _, dir := range extraBinds {
		isolateArgs = append(isolateArgs, "--dir="+dir+":rw:maybe")
	}

	// Pass through environment variables that block runtimes need to
	// locate their caches, tools, and locale settings.
	isolateArgs = append(isolateArgs,
		"--env=HOME",
		"--env=PATH",
		"--env=LANG",
		"--env=LC_ALL",
		"--env=TZ",
		// uv-managed Python location.  When set (e.g. pointed at a shared
		// volume so a venv built elsewhere resolves here), uv inside the
		// sandbox must see it too; the matching --dir bind is added by
		// languageSandboxBinds.
		"--env=UV_PYTHON_INSTALL_DIR",
	)

	// Redirect caches into the work directory.  The sandbox uid typically
	// differs from the host user, so writing to the user's real cache
	// directories fails with EACCES.  Each invocation gets its own cache
	// under the work dir which is fully writable.
	cacheDir := filepath.Join(workDir, ".cache")
	_ = os.MkdirAll(cacheDir, 0777)
	_ = os.Chmod(cacheDir, 0777)
	isolateArgs = append(isolateArgs,
		"--env=XDG_CACHE_HOME="+cacheDir,
		"--env=UV_CACHE_DIR="+filepath.Join(cacheDir, "uv"),
		"--env=PYTHONPYCACHEPREFIX="+filepath.Join(cacheDir, "pycache"),
	)

	// Point the runtime libraries at the block's manifest so they use
	// the declared output names (e.g. "vectors" plural) instead of
	// falling back to the type-default name (e.g. "vector" singular).
	manifestPath := filepath.Join(entry.InstalledPath, "blocks", entry.BlockName+".yaml")
	if _, err := os.Stat(manifestPath); err == nil {
		isolateArgs = append(isolateArgs, "--env=SPADE_BLOCK_MANIFEST="+manifestPath)
	}

	// Isolate's default is no network; opt-in for blocks that declared it.
	// Network-capable blocks also need DNS and TLS trust roots visible
	// inside the sandbox: /etc (resolv.conf, hosts, nsswitch.conf,
	// ssl/certs) plus the systemd-resolved stub on Ubuntu systems.
	if manifest.Network {
		isolateArgs = append(isolateArgs,
			"--share-net",
			"--dir=/etc",
			"--dir=/run/systemd/resolve:maybe",
		)
	}

	// Wrap the command in `sh -c 'umask 0000; exec <cmd>'` so files the
	// block creates inside the sandbox are world-readable/writable.
	// Without this, files owned by the sandbox uid (typically 100000)
	// cannot be cleaned up by the host user after the pipeline completes.
	shellCmd := "umask 0000; exec " + shellQuote(append([]string{execPath}, args...))
	isolateArgs = append(isolateArgs, "--run", "--", "/bin/sh", "-c", shellCmd)

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

// pythonEditableInstallPaths reads every .pth file under the block's .venv
// site-packages directory and returns the set of absolute paths they
// reference.  These paths need to be bound into the sandbox so editable
// installs (e.g. the spade runtime library under libs/python/src) resolve.
func pythonEditableInstallPaths(installedPath string) []string {
	seen := make(map[string]bool)
	// site-packages lives at .venv/lib/python<ver>/site-packages
	libDir := filepath.Join(installedPath, ".venv", "lib")
	entries, err := os.ReadDir(libDir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "python") {
			continue
		}
		sitePkgs := filepath.Join(libDir, e.Name(), "site-packages")
		pthFiles, err := filepath.Glob(filepath.Join(sitePkgs, "*.pth"))
		if err != nil {
			continue
		}
		for _, pth := range pthFiles {
			data, err := os.ReadFile(pth)
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "import ") {
					continue
				}
				if !filepath.IsAbs(line) {
					continue
				}
				if info, err := os.Stat(line); err == nil && info.IsDir() {
					seen[line] = true
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	return out
}

// shellQuote joins args into a single shell-safe command string.
func shellQuote(argv []string) string {
	var b strings.Builder
	for i, a := range argv {
		if i > 0 {
			b.WriteByte(' ')
		}
		// Single-quote and escape any embedded single quotes.
		b.WriteByte('\'')
		b.WriteString(strings.ReplaceAll(a, "'", `'\''`))
		b.WriteByte('\'')
	}
	return b.String()
}

// discoverArgPaths walks a block's args looking for filesystem paths (either
// bare absolute paths or file:// URIs, possibly containing a `*` glob) and
// returns the set of directories those paths live in.  Callers bind those
// directories into the sandbox so the block can read/write them.
func discoverArgPaths(args map[string]any) []string {
	seen := make(map[string]bool)
	for _, v := range args {
		s, ok := v.(string)
		if !ok {
			continue
		}
		// Normalise file:// and bare absolute paths to a filesystem path.
		p := s
		if strings.HasPrefix(p, "file://") {
			p = strings.TrimPrefix(p, "file://")
		} else if !strings.HasPrefix(p, "/") {
			continue
		}
		// Strip trailing glob segment: /dir/*.csv -> /dir
		if i := strings.IndexAny(p, "*?["); i >= 0 {
			p = p[:i]
		}
		// Climb to the nearest existing directory.
		dir := filepath.Dir(p)
		for dir != "/" && dir != "." {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				break
			}
			dir = filepath.Dir(dir)
		}
		if dir == "/" || dir == "." {
			continue
		}
		seen[dir] = true
	}
	binds := make([]string, 0, len(seen))
	for d := range seen {
		binds = append(binds, d)
	}
	return binds
}

// languageSandboxBinds returns the additional --dir arguments needed by the
// block's runtime toolchain: the interpreter binary and any caches it writes.
// Paths that may not exist use the :maybe modifier so isolate skips them
// rather than failing.
func languageSandboxBinds(entry BlockRegistryEntry) []string {
	var binds []string
	home := os.Getenv("HOME")

	// Bind the directory that contains a discovered binary, read-only.
	bindTool := func(name string) {
		if p, err := exec.LookPath(name); err == nil {
			if real, err := filepath.EvalSymlinks(p); err == nil {
				p = real
			}
			binds = append(binds, "--dir="+filepath.Dir(p))
		}
	}

	switch CollectionLanguage(entry.Language) {
	case CollectionLanguagePython:
		// uv itself, uv-managed Python installs, and uv's cache.
		bindTool("uv")
		if home != "" {
			binds = append(binds,
				"--dir="+filepath.Join(home, ".local/share/uv")+":maybe",
				"--dir="+filepath.Join(home, ".cache/uv")+":rw:maybe",
				"--dir="+filepath.Join(home, ".local/state/uv")+":rw:maybe",
			)
		}
		// When uv-managed Python lives outside $HOME (e.g. on a shared
		// volume via UV_PYTHON_INSTALL_DIR), bind that directory too so the
		// venv's base interpreter resolves inside the sandbox.
		if pyDir := os.Getenv("UV_PYTHON_INSTALL_DIR"); pyDir != "" {
			binds = append(binds, "--dir="+pyDir+":maybe")
		}
		// Editable installs in the .venv point at directories outside
		// the sandbox via .pth files; bind those directories too so
		// `import spade` etc. resolves inside the sandbox.
		for _, p := range pythonEditableInstallPaths(entry.InstalledPath) {
			binds = append(binds, "--dir="+p+":maybe")
		}
	case CollectionLanguageR:
		bindTool("Rscript")
		if home != "" {
			binds = append(binds,
				"--dir="+filepath.Join(home, ".local/share/R")+":maybe",
				"--dir="+filepath.Join(home, "R")+":maybe",
			)
		}
	case CollectionLanguageTypeScript:
		bindTool("bun")
		if home != "" {
			binds = append(binds,
				"--dir="+filepath.Join(home, ".bun")+":maybe",
			)
		}
	}

	return binds
}
