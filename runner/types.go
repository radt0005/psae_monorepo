// Package spade_runner provides the worker-side orchestration and transport
// for the Spade block execution system.  The library is consumed by both the
// spade-worker binary (for RabbitMQ-driven cloud/multi-worker operation) and
// the CLI's local single-instance mode (spade run).
//
// All block-execution primitives — invocation directory setup, input symlinking
// (including map/reduce), subprocess execution under the isolate sandbox,
// registry access, manifest parsing, hashing, and caching — live in
// ../core/.  This package adds only the transport and orchestration glue
// on top.
package spade_runner

import (
	"core"
	"fmt"

	"github.com/google/uuid"
)

// Job is the payload the worker consumes from the spade.jobs queue.
//
// It is a superset of core.WorkerAssignment: the scheduler must include
// the owning pipeline and the manifests of every block the assignment
// references (the current block plus every direct dependency) so the
// worker can resolve inputs and set up symlinks without a round-trip.
//
// Wire format: JSON, durable.  Published by the scheduler on the
// spade.jobs queue (see ../spec/worker.md §Communication).
type Job struct {
	// Assignment carries the core invocation identity: invocation ID,
	// block name, pipeline ID, work dir, args, and input references.
	Assignment core.WorkerAssignment `json:"assignment"`

	// Pipeline is the full pipeline definition.  The worker uses it to
	// locate the PipelineBlock for this invocation and to walk dependency
	// IDs back to block names during input resolution.
	Pipeline core.Pipeline `json:"pipeline"`

	// Manifests is a lookup of block name → parsed BlockManifest, covering
	// at minimum the current block and every direct dependency.
	// Populated by the scheduler at dispatch time.
	Manifests map[string]core.BlockManifest `json:"manifests"`

	// CapabilityToken is a short-lived, scheduler-signed token scoping secret
	// access to exactly the secrets this invocation's block declared (see
	// spec/secrets.md §6). The worker relays it to the KMS /resolve endpoint.
	// Empty when the block declares no secrets.
	CapabilityToken string `json:"capability_token,omitempty"`
}

// ParseInvocationID splits an invocation ID of the form
// "<uuid>[.<i>[.<j>…]]" into the block UUID and its map index vector
// (nil for non-mapped invocations).  It delegates to core.ParseInvocationID.
func ParseInvocationID(id string) (uuid.UUID, []int, error) {
	return core.ParseInvocationID(id)
}

// InvocationFromJob converts a Job's WorkerAssignment + Pipeline context
// into the core.BlockInvocation accepted by core.Execute.
//
// It parses the invocation ID for a possible map-index suffix, looks up
// the matching PipelineBlock so Arguments and Inputs come from the
// authoritative pipeline definition (not from the assignment payload,
// which could drift), and returns a fully populated invocation.
func InvocationFromJob(j Job) (core.BlockInvocation, error) {
	blockID, mapIndices, err := ParseInvocationID(j.Assignment.InvocationID)
	if err != nil {
		return core.BlockInvocation{}, err
	}

	for _, pb := range j.Pipeline.Blocks {
		if pb.Id != blockID {
			continue
		}
		inv := core.BlockInvocation{
			Id:         pb.Id,
			BlockId:    pb.Name,
			PipelineId: j.Assignment.PipelineID,
			Inputs:     pb.Inputs,
			Arguments:  pb.Args,
			MapIndices: mapIndices,
		}
		return inv, nil
	}
	return core.BlockInvocation{}, fmt.Errorf("invocation %s references block %s not in pipeline",
		j.Assignment.InvocationID, blockID)
}

// DependencyManifests builds the dependency-name → manifest map that
// core.ResolveInputs needs.  It walks the invocation's input refs,
// resolves each to the referenced PipelineBlock's name, and looks up
// the manifest from the Job's Manifests map.
//
// Returns an error if any referenced dependency block cannot be
// resolved to a manifest (which means the scheduler dispatched an
// incomplete Job — a protocol-level error, not a user-pipeline error).
func DependencyManifests(j Job, inv core.BlockInvocation) (map[uuid.UUID]core.BlockManifest, error) {
	out := make(map[uuid.UUID]core.BlockManifest)
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
		if _, already := out[depID]; already {
			continue
		}
		var depName string
		for _, pb := range j.Pipeline.Blocks {
			if pb.Id == depID {
				depName = pb.Name
				break
			}
		}
		if depName == "" {
			return nil, fmt.Errorf("dependency block %s not found in pipeline", depID)
		}
		m, ok := j.Manifests[depName]
		if !ok {
			return nil, fmt.Errorf("no manifest for dependency block %q (%s)", depName, depID)
		}
		out[depID] = m
	}
	return out, nil
}
