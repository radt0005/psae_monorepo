package spade_runner

import (
	"core"

	"github.com/google/uuid"
)

// BuildJob is the scheduler-side counterpart to InvocationFromJob.  It
// assembles the spade.jobs payload (Job envelope) from an outgoing
// WorkerAssignment, the owning pipeline, and the manifests of every
// block the assignment references (the current block plus every direct
// dependency).
//
// The scheduling server calls this when it pulls a ready
// BlockInvocation from core.MultiTenantScheduler.Drain() and wants to
// publish it on spade.jobs.  See the scheduling server's
// IMPLEMENTATION_PLAN.md Phase 3.6 / 3.7 for the motivation.
func BuildJob(assignment core.WorkerAssignment, pipeline core.Pipeline, manifests map[string]core.BlockManifest) Job {
	// Make a defensive copy of the manifest map so the caller is free
	// to mutate its own map after the call without surprising the
	// receiver across the wire.
	mcopy := make(map[string]core.BlockManifest, len(manifests))
	for k, v := range manifests {
		mcopy[k] = v
	}
	return Job{
		Assignment: assignment,
		Pipeline:   pipeline,
		Manifests:  mcopy,
	}
}

// BuildJobForInvocation is a convenience that fills out a
// core.WorkerAssignment from a fully populated core.BlockInvocation,
// then delegates to BuildJob.  workDir is the worker-side working
// directory for the pipeline; the scheduler-side caller may pass an
// empty string to let the worker derive it from its own work root.
func BuildJobForInvocation(inv core.BlockInvocation, pipeline core.Pipeline, manifests map[string]core.BlockManifest, workDir string) Job {
	assignment := core.WorkerAssignment{
		InvocationID: inv.InvocationID(),
		BlockName:    inv.BlockId,
		PipelineID:   pipeline.Id,
		WorkDir:      workDir,
		Args:         inv.Arguments,
		Inputs:       inv.Inputs,
	}
	// Carry the pinned collection version (Option A) from the owning pipeline
	// block (matched by Id; mapped invocations share the base block Id). Empty
	// when unset, which keeps legacy latest-installed lookup.
	for _, pb := range pipeline.Blocks {
		if pb.Id == inv.Id {
			assignment.CollectionVersion = pb.Version
			break
		}
	}
	// Ensure manifests include every direct dependency, looked up
	// through the pipeline.  Callers can pre-fill manifests; this
	// helper only fills in what's missing if the caller passed a
	// map that already covers every dep.
	for _, ref := range inv.Inputs {
		var depID uuid.UUID
		if ref.Block != nil {
			depID = *ref.Block
		} else {
			depID = ref.ID
		}
		if depID == uuid.Nil {
			continue
		}
		for _, pb := range pipeline.Blocks {
			if pb.Id == depID {
				if _, has := manifests[pb.Name]; !has {
					// caller did not supply this manifest;
					// leave the field empty so the worker
					// can return a meaningful error.
				}
				break
			}
		}
	}
	return BuildJob(assignment, pipeline, manifests)
}
