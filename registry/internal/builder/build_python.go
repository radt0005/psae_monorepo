package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// PythonBuilder builds a Python collection into a self-contained, relocatable
// artifact: the source tree plus a populated `.venv` with the locked
// dependencies installed (registry.md §5, "the source tree, the uv lockfile,
// and a populated dependency cache ... the worker unpacks and runs; no further
// uv sync is required"). The worker runs blocks with
// `uv run --project <dir> --no-sync -m <module>.<block>` (core.ResolveEntrypoint),
// so the environment must already be complete and must survive being unpacked to
// a different path than it was built at.
//
// Relocatability rests on two things:
//   - The collection package is installed **non-editable** (`--no-editable`), so
//     it is copied into the venv's site-packages rather than referenced by an
//     absolute `.pth` path that would break when the artifact moves.
//   - The base interpreter is the image's system `python3`. Because the bundler
//     image is the worker base image plus toolchains (registry.md §5), that
//     interpreter lives at the same absolute path on the worker, so the venv's
//     `pyvenv.cfg`/symlinks resolve after relocation.
type PythonBuilder struct{}

func (PythonBuilder) Build(ctx context.Context, srcDir, collection, version string) (string, error) {
	artifactDir, err := os.MkdirTemp("", "spade-artifact-*")
	if err != nil {
		return "", err
	}

	// Ship the whole source tree (handlers, pyproject.toml, uv.lock) so
	// `uv run --project` finds the project on the worker.
	if err := copyTree(srcDir, artifactDir); err != nil {
		os.RemoveAll(artifactDir)
		return "", fmt.Errorf("copying source tree: %w", err)
	}

	// uv sync requires a lockfile; real collections commit uv.lock (spade check
	// enforces it). Generate one if it is somehow absent so the build still
	// produces a reproducible environment.
	env := append(os.Environ(),
		"UV_PYTHON_PREFERENCE=only-system", // never download a managed interpreter
		"UV_PROJECT_ENVIRONMENT="+filepath.Join(artifactDir, ".venv"),
	)
	if _, err := os.Stat(filepath.Join(artifactDir, "uv.lock")); os.IsNotExist(err) {
		if out, err := runInEnv(ctx, artifactDir, env, "uv", "lock"); err != nil {
			os.RemoveAll(artifactDir)
			return "", fmt.Errorf("uv lock failed: %v\n%s", err, out)
		}
	}

	// Install the locked deps and the collection itself, non-editable, into the
	// shipped venv. --frozen guarantees we build exactly the screened lockfile.
	if out, err := runInEnv(ctx, artifactDir, env,
		"uv", "sync", "--frozen", "--no-editable", "--no-dev"); err != nil {
		os.RemoveAll(artifactDir)
		return "", fmt.Errorf("uv sync failed: %v\n%s", err, out)
	}

	return artifactDir, nil
}
