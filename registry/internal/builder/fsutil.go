package builder

import (
	"io"
	"os"
	"path/filepath"
)

// copyTree recursively copies the contents of src into dst, preserving the
// executable bit and skipping version-control and prior build-environment
// directories (.git, .venv, renv/library, node_modules, target) so a rebuild is
// clean and the copied source tree doesn't carry a non-relocatable environment
// from the clone. dst is created if needed.
func copyTree(src, dst string) error {
	skip := map[string]bool{
		".git": true, ".venv": true, "node_modules": true, "target": true,
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		// Skip well-known build/VCS directories at any depth.
		if info.IsDir() && skip[info.Name()] {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Resolve and copy the target's bytes rather than the link, so the
			// copied tree has no dangling links into the build workspace.
			real, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil // skip broken links
			}
			ri, err := os.Stat(real)
			if err != nil || ri.IsDir() {
				return nil
			}
			return copyFileMode(real, target, 0o644)
		}
		mode := os.FileMode(0o644)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o755
		}
		return copyFileMode(path, target, mode)
	})
}

// copyFileMode copies src to dst with the given mode, creating parent dirs.
func copyFileMode(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
