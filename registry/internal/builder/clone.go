package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Cloner acquires source at a specific commit into a fresh directory.
type Cloner interface {
	Clone(ctx context.Context, repoURL, commitSHA string) (dir string, cleanup func(), err error)
}

// GitCloner clones via the `git` CLI (present in every builder container).
type GitCloner struct{}

// Clone clones repoURL into a temp dir and checks out commitSHA. It clones then
// checks out (rather than fetching the bare SHA) for compatibility with servers
// that do not allow fetching arbitrary SHAs, and with local file:// repos used
// in tests.
func (GitCloner) Clone(ctx context.Context, repoURL, commitSHA string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "spade-src-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }

	clone := exec.CommandContext(ctx, "git", "clone", "--no-checkout", repoURL, dir)
	if out, err := clone.CombinedOutput(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("git clone %s: %v\n%s", repoURL, err, out)
	}

	checkout := exec.CommandContext(ctx, "git", "-C", dir, "checkout", "--detach", commitSHA)
	if out, err := checkout.CombinedOutput(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("git checkout %s: %v\n%s", commitSHA, err, out)
	}
	return dir, cleanup, nil
}
