package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RBuilder builds an R collection into a self-contained artifact: the source
// tree plus a populated R library shipped *inside* the artifact (registry.md §5,
// "the source tree, the lockfile, and a populated library"). The worker has no R
// toolchain, so dependencies must be baked in at build time, into an
// artifact-local library at <artifact>/renv/library.
//
// Dependencies are declared one of several ways; the builder picks the first
// that applies (see PAK_ADOPTION_PLAN.md):
//
//   - pak manifest — a DESCRIPTION (Imports:) and/or a committed pak `pkg.lock`.
//     This is the primary path: pak installs from the lock (or resolves the
//     DESCRIPTION into a fresh lock first), pulling P3M binaries and writing real
//     package files into the library (no cache symlinks, so it packages cleanly).
//   - setup.R — a supported fallback for collections that declare deps
//     imperatively; run with the library on R's search path so its installs land
//     in the artifact.
//   - renv.lock (rollback-only legacy) — `renv::restore` into the library.
//   - otherwise base R only; ship as-is.
//
// NOTE: making the worker put this shipped library on R's search path is a
// deferred, coordinated change to core/executor.go. See
// BUILDERS_IMPLEMENTATION_PLAN.md Phase E / notes.md §C2.
type RBuilder struct{}

func (RBuilder) Build(ctx context.Context, srcDir, collection, version string) (string, error) {
	artifactDir, err := os.MkdirTemp("", "spade-artifact-*")
	if err != nil {
		return "", err
	}

	if err := copyTree(srcDir, artifactDir); err != nil {
		os.RemoveAll(artifactDir)
		return "", fmt.Errorf("copying source tree: %w", err)
	}

	// The artifact-local R library the worker will (in future) put on the search
	// path. Every dependency strategy installs into it.
	libDir := filepath.Join(artifactDir, "renv", "library")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		os.RemoveAll(artifactDir)
		return "", err
	}

	hasPkgLock := fileExists(filepath.Join(artifactDir, "pkg.lock"))
	hasDescription := fileExists(filepath.Join(artifactDir, "DESCRIPTION"))
	hasSetup := fileExists(filepath.Join(artifactDir, "setup.R"))
	populated, err := renvLockHasPackages(filepath.Join(artifactDir, "renv.lock"))
	if err != nil {
		os.RemoveAll(artifactDir)
		return "", err
	}

	switch {
	case hasPkgLock || hasDescription:
		if err := pakInstall(ctx, artifactDir, libDir, hasPkgLock); err != nil {
			os.RemoveAll(artifactDir)
			return "", err
		}
	case hasSetup:
		// Supported fallback: run setup.R with R_LIBS_USER pointed at the artifact
		// library so its installs (whether install.packages or pak::pkg_install)
		// are captured in the artifact.
		if out, err := runInEnv(ctx, artifactDir, rEnv(libDir), "Rscript", "setup.R"); err != nil {
			os.RemoveAll(artifactDir)
			return "", fmt.Errorf("setup.R failed: %v\n%s", err, out)
		}
	case populated:
		// Legacy rollback path: restore the locked packages via renv.
		script := fmt.Sprintf(
			`.libPaths(%q); renv::restore(lockfile=%q, library=%q, prompt=FALSE)`,
			libDir,
			filepath.Join(artifactDir, "renv.lock"),
			libDir,
		)
		if out, err := runInEnv(ctx, artifactDir, rEnv(libDir), "Rscript", "-e", script); err != nil {
			os.RemoveAll(artifactDir)
			return "", fmt.Errorf("renv::restore failed: %v\n%s", err, out)
		}
	default:
		// Base-R-only collection: nothing to install.
	}

	return artifactDir, nil
}

// pakInstall installs the collection's R dependencies into libDir using pak.
//
// With a committed pkg.lock it installs from it verbatim (reproducible). Without
// one it resolves the DESCRIPTION's deps (`deps::.`) into a fresh pkg.lock first,
// mirroring build_python.go generating uv.lock when absent. The Rscript runs with
// CWD=artifactDir (via runInEnv), so the `deps::.` ref and the pkg.lock path stay
// relative and the committed lock remains portable (PAK_ADOPTION_PLAN R0-b).
//
// PPM binary-repo selection is configured in the bundler image (Rprofile.site),
// not here, so this code stays portable across build hosts.
func pakInstall(ctx context.Context, artifactDir, libDir string, hasPkgLock bool) error {
	var script string
	if hasPkgLock {
		script = fmt.Sprintf(
			`.libPaths(%[1]q); pak::lockfile_install("pkg.lock", lib=%[1]q)`,
			libDir,
		)
	} else {
		script = fmt.Sprintf(
			`.libPaths(%[1]q); `+
				`pak::lockfile_create("deps::.", lockfile="pkg.lock", lib=%[1]q); `+
				`pak::lockfile_install("pkg.lock", lib=%[1]q)`,
			libDir,
		)
	}
	if out, err := runInEnv(ctx, artifactDir, rEnv(libDir), "Rscript", "-e", script); err != nil {
		return fmt.Errorf("pak install failed: %v\n%s", err, out)
	}

	// D6 interim: the spade runtime lib is not yet published, so install it from
	// source when its location is provided (SPADE_R_LIB_SRC), mirroring setup.R.
	if err := pakInstallSpade(ctx, artifactDir, libDir); err != nil {
		return err
	}

	// R0-a: pak leaves a `_cache` staging dir (install locks) in the library. It
	// has no runtime use; drop it so it isn't packaged into the artifact.
	if err := os.RemoveAll(filepath.Join(libDir, "_cache")); err != nil {
		return fmt.Errorf("removing pak _cache: %w", err)
	}
	return nil
}

// pakInstallSpade installs the local spade runtime package into libDir when its
// source location is supplied via SPADE_R_LIB_SRC (the same convention setup.R
// uses). A no-op when unset or missing: deps-only collections don't need it, and
// once spade is published it moves into DESCRIPTION Imports and this drops out.
func pakInstallSpade(ctx context.Context, artifactDir, libDir string) error {
	src := os.Getenv("SPADE_R_LIB_SRC")
	if src == "" || !fileExists(src) {
		return nil
	}
	script := fmt.Sprintf(
		`.libPaths(%q); pak::local_install(%q, lib=%q, upgrade=FALSE, ask=FALSE)`,
		libDir, src, libDir,
	)
	if out, err := runInEnv(ctx, artifactDir, rEnv(libDir), "Rscript", "-e", script); err != nil {
		return fmt.Errorf("installing local spade from %s: %v\n%s", src, err, out)
	}
	return nil
}

// rEnv builds the environment for R build steps, pointing every library
// discovery variable at the artifact-local library so packages land there.
func rEnv(libDir string) []string {
	return append(os.Environ(),
		"R_LIBS_USER="+libDir,
		"R_LIBS="+libDir,
	)
}

// renvLockHasPackages reports whether an renv.lock exists and lists at least one
// package. A missing lockfile is not an error (the collection may use setup.R).
func renvLockHasPackages(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var lock struct {
		Packages map[string]json.RawMessage `json:"Packages"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return false, fmt.Errorf("parsing renv.lock: %w", err)
	}
	return len(lock.Packages) > 0, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
