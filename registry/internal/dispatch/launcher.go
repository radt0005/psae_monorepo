// Package dispatch contains the build dispatcher: it watches the build queue,
// mints a per-job builder token, and launches an ephemeral, language-specific
// build container per build (the build worker never touches the database). The
// launcher is abstracted so tests run without Docker.
package dispatch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Launcher runs one build to completion in an isolated environment. env carries
// BUILD_JOB_ID, BUILD_TOKEN, REGISTRY_URL, and the staging-scoped S3_* vars; no
// database credentials are ever passed.
type Launcher interface {
	Run(ctx context.Context, image string, env map[string]string) error
}

// DockerLauncher launches a fresh container per build via `docker run --rm`,
// giving each build a clean environment (registry.md §5.2 / hosting.md §5.2).
// It shells out to the docker CLI to avoid a heavy SDK dependency.
type DockerLauncher struct {
	// DockerBin is the docker executable (default "docker").
	DockerBin string
	// ExtraArgs are appended to `docker run` (e.g. network, mounts).
	ExtraArgs []string
}

// Run executes `docker run --rm -e ... <image>` and waits for it to exit.
func (d DockerLauncher) Run(ctx context.Context, image string, env map[string]string) error {
	bin := d.DockerBin
	if bin == "" {
		bin = "docker"
	}
	args := []string{"run", "--rm"}
	for k, v := range env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, d.ExtraArgs...)
	args = append(args, image)

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = os.Stderr // build logs surface in the registry's stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker run %s: %w", image, err)
	}
	return nil
}

// InProcessLauncher runs the build flow in-process instead of in a container.
// It is used by tests and by the single-binary local mode; it does not provide
// container isolation. The actual build logic is supplied by Runner so this
// package does not import internal/builder directly (avoiding heavy coupling).
type InProcessLauncher struct {
	// Runner performs a build given the job's env. It mirrors what the
	// containerized builder binary does with the same env vars.
	Runner func(ctx context.Context, env map[string]string) error
}

// Run invokes the configured Runner.
func (l InProcessLauncher) Run(ctx context.Context, image string, env map[string]string) error {
	if l.Runner == nil {
		return fmt.Errorf("InProcessLauncher: no Runner configured")
	}
	return l.Runner(ctx, env)
}
