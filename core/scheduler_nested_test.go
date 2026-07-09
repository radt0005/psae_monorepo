package core

import (
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"
)

// runSchedulerSim drives a SinglePipelineScheduler to completion in a
// single-threaded loop, fabricating results.  Map blocks report a
// StatusMap result whose expansion is produced by expansionFor (keyed by
// the invocation ID); every other block reports StatusComplete.  If
// failOn matches an invocation ID, that invocation reports an error.
// Returns the invocation IDs in execution order and whether the pipeline
// ran to completion.
func runSchedulerSim(t *testing.T, s *SinglePipelineScheduler, p Pipeline, manifests map[string]BlockManifest, expansionFor func(invID string) []ExpansionItem, failOn string) ([]string, bool) {
	t.Helper()
	blockNameKind := func(name string) BlockKind {
		if m, ok := manifests[name]; ok {
			return m.Kind
		}
		return BlockKindStandard
	}

	var executed []string
	for steps := 0; ; steps++ {
		if steps > 1000 {
			t.Fatalf("simulation did not terminate; executed=%v pending=%d executable=%d", executed, len(s.PendingBlocks), len(s.ExecutableBlocks))
		}
		inv, done, err := s.Next()
		if err != nil {
			t.Fatalf("scheduler error: %v", err)
		}
		if done {
			// Next() also reports done after a halt (queues cleared);
			// completion means done without cancellation.
			return executed, !s.Cancelled
		}
		if inv.BlockId == "" {
			if s.Cancelled {
				return executed, false
			}
			t.Fatalf("scheduler stalled: nothing executable but %d pending remain; executed=%v pending=%v", len(s.PendingBlocks), executed, pendingKeys(s))
		}
		invID := inv.InvocationID()
		executed = append(executed, invID)

		res := BlockInvocationResult{
			Id:         inv.Id,
			PipelineId: p.Id,
			MapIndices: inv.MapIndices,
		}
		switch {
		case invID == failOn:
			res.Status = ExecutionStatusError
			res.Error = "simulated failure"
		case blockNameKind(inv.BlockId) == BlockKindMap:
			res.Status = ExecutionStatusMap
			res.Expansion = &ExpansionManifest{Items: expansionFor(invID)}
		default:
			res.Status = ExecutionStatusComplete
		}
		if err := s.Update(res); err != nil {
			t.Fatalf("update error: %v", err)
		}
	}
}

func pendingKeys(s *SinglePipelineScheduler) []string {
	var keys []string
	for k := range s.PendingBlocks {
		keys = append(keys, k)
	}
	return keys
}

func nItems(n int) []ExpansionItem {
	items := make([]ExpansionItem, n)
	for i := range items {
		items[i] = ExpansionItem{Path: fmt.Sprintf("item_%03d", i), Key: fmt.Sprintf("item_%03d", i)}
	}
	return items
}

// newNestedScheduler builds the scheduler for a fixture pipeline and runs
// IdentifyMapContexts.
func newNestedScheduler(t *testing.T, blocks []ctBlock) (*SinglePipelineScheduler, Pipeline, map[string]BlockManifest, map[string]uuid.UUID) {
	t.Helper()
	p, manifests, _, ids := buildContextTreeFixture(t, blocks)
	s := NewSchedulerForPipeline(p)
	s.Manifests = manifests
	if err := s.IdentifyMapContexts(); err != nil {
		t.Fatalf("IdentifyMapContexts: %v", err)
	}
	return s, p, manifests, ids
}

// TestNestedMapReduceHappyPath: depth-2 chain with ragged inner counts
// (outer item 0 → 3 inner items, outer item 1 → 1 inner item).
func TestNestedMapReduceHappyPath(t *testing.T) {
	s, p, manifests, ids := newNestedScheduler(t, []ctBlock{
		{"src", BlockKindStandard, nil},
		{"M1", BlockKindMap, []string{"src"}},
		{"M2", BlockKindMap, []string{"M1"}},
		{"X", BlockKindStandard, []string{"M2"}},
		{"R2", BlockKindReduce, []string{"X"}},
		{"R1", BlockKindReduce, []string{"R2"}},
		{"final", BlockKindStandard, []string{"R1"}},
	})

	id := func(name string, indices ...int) string {
		return FormatInvocationID(ids[name], indices)
	}
	expansions := map[string][]ExpansionItem{
		id("M1"):    nItems(2),
		id("M2", 0): nItems(3),
		id("M2", 1): nItems(1),
	}

	executed, completed := runSchedulerSim(t, s, p, manifests, func(invID string) []ExpansionItem {
		items, ok := expansions[invID]
		if !ok {
			t.Fatalf("unexpected map invocation %s", invID)
		}
		return items
	}, "")

	if !completed {
		t.Fatal("pipeline did not complete")
	}

	want := []string{
		id("src"), id("M1"),
		id("M2", 0), id("M2", 1),
		id("X", 0, 0), id("X", 0, 1), id("X", 0, 2),
		id("X", 1, 0),
		id("R2", 0), id("R2", 1),
		id("R1"), id("final"),
	}
	if len(executed) != len(want) {
		t.Fatalf("executed %d invocations, want %d: %v", len(executed), len(want), executed)
	}
	for _, w := range want {
		if !slices.Contains(executed, w) {
			t.Errorf("missing invocation %s in %v", w, executed)
		}
	}

	// Ordering: each inner reduce instance after all its own siblings;
	// the outer reduce after both inner reduces; final last.
	pos := make(map[string]int, len(executed))
	for i, e := range executed {
		pos[e] = i
	}
	after := func(a, b string) {
		if pos[a] <= pos[b] {
			t.Errorf("%s ran at %d, expected after %s at %d", a, pos[a], b, pos[b])
		}
	}
	after(id("R2", 0), id("X", 0, 0))
	after(id("R2", 0), id("X", 0, 1))
	after(id("R2", 0), id("X", 0, 2))
	after(id("R2", 1), id("X", 1, 0))
	after(id("R1"), id("R2", 0))
	after(id("R1"), id("R2", 1))
	after(id("final"), id("R1"))
}

// TestNestedMapEmptyInnerExpansion: one outer item legitimately expands to
// zero inner items — its inner reduce still runs (with an empty
// collection) and the pipeline completes.
func TestNestedMapEmptyInnerExpansion(t *testing.T) {
	s, p, manifests, ids := newNestedScheduler(t, []ctBlock{
		{"M1", BlockKindMap, nil},
		{"M2", BlockKindMap, []string{"M1"}},
		{"X", BlockKindStandard, []string{"M2"}},
		{"R2", BlockKindReduce, []string{"X"}},
		{"R1", BlockKindReduce, []string{"R2"}},
	})

	id := func(name string, indices ...int) string {
		return FormatInvocationID(ids[name], indices)
	}
	expansions := map[string][]ExpansionItem{
		id("M1"):    nItems(2),
		id("M2", 0): nItems(2),
		id("M2", 1): nItems(0), // empty instance
	}

	executed, completed := runSchedulerSim(t, s, p, manifests, func(invID string) []ExpansionItem {
		return expansions[invID]
	}, "")

	if !completed {
		t.Fatal("pipeline did not complete")
	}
	if slices.Contains(executed, id("X", 1, 0)) {
		t.Error("no X invocations should exist for the empty inner instance")
	}
	for _, w := range []string{id("R2", 0), id("R2", 1), id("R1")} {
		if !slices.Contains(executed, w) {
			t.Errorf("missing %s; executed=%v", w, executed)
		}
	}
}

// TestMapEmptyTopLevelExpansion: a top-level map that expands to zero
// items must not stall the pipeline (regression: HandleMap used to return
// early without recording anything).
func TestMapEmptyTopLevelExpansion(t *testing.T) {
	s, p, manifests, ids := newNestedScheduler(t, []ctBlock{
		{"M", BlockKindMap, nil},
		{"X", BlockKindStandard, []string{"M"}},
		{"R", BlockKindReduce, []string{"X"}},
		{"final", BlockKindStandard, []string{"R"}},
	})

	id := func(name string, indices ...int) string {
		return FormatInvocationID(ids[name], indices)
	}

	executed, completed := runSchedulerSim(t, s, p, manifests, func(invID string) []ExpansionItem {
		return nil
	}, "")

	if !completed {
		t.Fatal("pipeline did not complete with an empty top-level expansion")
	}
	for _, w := range []string{id("M"), id("R"), id("final")} {
		if !slices.Contains(executed, w) {
			t.Errorf("missing %s; executed=%v", w, executed)
		}
	}
	if len(executed) != 3 {
		t.Errorf("expected exactly M, R, final; got %v", executed)
	}
}

// TestNestedMapBroadcasts: a depth-0 block and a depth-1 block both feed a
// depth-2 block; each X invocation must wait for the correct instance of
// each broadcast dependency.
func TestNestedMapBroadcasts(t *testing.T) {
	s, p, manifests, ids := newNestedScheduler(t, []ctBlock{
		{"model", BlockKindStandard, nil},
		{"M1", BlockKindMap, nil},
		{"ref", BlockKindStandard, []string{"M1"}},
		{"M2", BlockKindMap, []string{"M1"}},
		{"X", BlockKindStandard, []string{"M2", "ref", "model"}},
		{"R2", BlockKindReduce, []string{"X"}},
		{"R1", BlockKindReduce, []string{"R2"}},
	})

	id := func(name string, indices ...int) string {
		return FormatInvocationID(ids[name], indices)
	}
	expansions := map[string][]ExpansionItem{
		id("M1"):    nItems(2),
		id("M2", 0): nItems(1),
		id("M2", 1): nItems(2),
	}

	executed, completed := runSchedulerSim(t, s, p, manifests, func(invID string) []ExpansionItem {
		return expansions[invID]
	}, "")

	if !completed {
		t.Fatal("pipeline did not complete")
	}

	pos := make(map[string]int, len(executed))
	for i, e := range executed {
		pos[e] = i
	}
	for _, x := range []string{id("X", 0, 0), id("X", 1, 0), id("X", 1, 1)} {
		if _, ok := pos[x]; !ok {
			t.Fatalf("missing %s; executed=%v", x, executed)
		}
		if pos[x] < pos[id("model")] {
			t.Errorf("%s ran before broadcast dependency model", x)
		}
	}
	// Per-instance broadcast: X.0.* waits on ref.0, X.1.* on ref.1.
	if pos[id("X", 0, 0)] < pos[id("ref", 0)] {
		t.Error("X.0.0 ran before ref.0")
	}
	if pos[id("X", 1, 0)] < pos[id("ref", 1)] || pos[id("X", 1, 1)] < pos[id("ref", 1)] {
		t.Error("X.1.* ran before ref.1")
	}
}

// TestNestedMapFailureHalts: a failure in one deep invocation halts the
// whole pipeline.
func TestNestedMapFailureHalts(t *testing.T) {
	s, p, manifests, ids := newNestedScheduler(t, []ctBlock{
		{"M1", BlockKindMap, nil},
		{"M2", BlockKindMap, []string{"M1"}},
		{"X", BlockKindStandard, []string{"M2"}},
		{"R2", BlockKindReduce, []string{"X"}},
		{"R1", BlockKindReduce, []string{"R2"}},
	})

	id := func(name string, indices ...int) string {
		return FormatInvocationID(ids[name], indices)
	}
	expansions := map[string][]ExpansionItem{
		id("M1"):    nItems(2),
		id("M2", 0): nItems(2),
		id("M2", 1): nItems(2),
	}

	_, completed := runSchedulerSim(t, s, p, manifests, func(invID string) []ExpansionItem {
		return expansions[invID]
	}, id("X", 0, 1))

	if completed {
		t.Fatal("pipeline should have halted on failure")
	}
	if !s.Cancelled {
		t.Error("scheduler should be cancelled after failure")
	}
	if len(s.PendingBlocks) != 0 || len(s.ExecutableBlocks) != 0 {
		t.Error("pending and executable should be cleared after failure")
	}
}

// TestSequentialAfterNested: a plain map/reduce pair following a nested
// pair schedules correctly.
func TestSequentialAfterNested(t *testing.T) {
	s, p, manifests, ids := newNestedScheduler(t, []ctBlock{
		{"M1", BlockKindMap, nil},
		{"M2", BlockKindMap, []string{"M1"}},
		{"X", BlockKindStandard, []string{"M2"}},
		{"R2", BlockKindReduce, []string{"X"}},
		{"R1", BlockKindReduce, []string{"R2"}},
		{"Mb", BlockKindMap, []string{"R1"}},
		{"Y", BlockKindStandard, []string{"Mb"}},
		{"Rb", BlockKindReduce, []string{"Y"}},
	})

	id := func(name string, indices ...int) string {
		return FormatInvocationID(ids[name], indices)
	}
	expansions := map[string][]ExpansionItem{
		id("M1"):    nItems(1),
		id("M2", 0): nItems(1),
		id("Mb"):    nItems(2),
	}

	executed, completed := runSchedulerSim(t, s, p, manifests, func(invID string) []ExpansionItem {
		return expansions[invID]
	}, "")

	if !completed {
		t.Fatal("pipeline did not complete")
	}
	for _, w := range []string{id("Y", 0), id("Y", 1), id("Rb")} {
		if !slices.Contains(executed, w) {
			t.Errorf("missing %s; executed=%v", w, executed)
		}
	}
}
