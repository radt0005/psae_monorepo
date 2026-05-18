package engine

import (
	"context"
	"testing"

	"core"

	"github.com/google/uuid"
	spade "spade_runner"

	"spade_server/store"
)

// mapReducePipeline builds a 4-block pipeline: src → map → middle → reduce.
// The map block fans out into N invocations of middle, then reduce gathers.
func mapReducePipeline(t *testing.T, mp *MapManifestProvider) (core.Pipeline, []uuid.UUID) {
	t.Helper()
	mp.Set("src", core.BlockManifest{
		ID:      "test.src",
		Outputs: map[string]core.OutputDeclaration{"tiles": {Type: "collection", ItemType: "file"}},
	})
	mp.Set("mapper", core.BlockManifest{
		ID:      "test.mapper",
		Kind:    core.BlockKindMap,
		Inputs:  map[string]core.InputDeclaration{"source": {Type: "collection", ItemType: "file"}},
		Outputs: map[string]core.OutputDeclaration{"manifest": {Type: "expansion"}},
	})
	mp.Set("middle", core.BlockManifest{
		ID:      "test.middle",
		Inputs:  map[string]core.InputDeclaration{"in": {Type: "file"}},
		Outputs: map[string]core.OutputDeclaration{"out": {Type: "file"}},
	})
	mp.Set("reducer", core.BlockManifest{
		ID:      "test.reducer",
		Kind:    core.BlockKindReduce,
		Inputs:  map[string]core.InputDeclaration{"items": {Type: "collection", ItemType: "file"}},
		Outputs: map[string]core.OutputDeclaration{"final": {Type: "file"}},
	})
	a := uuid.Must(uuid.NewV7())
	m := uuid.Must(uuid.NewV7())
	mid := uuid.Must(uuid.NewV7())
	red := uuid.Must(uuid.NewV7())
	p := core.Pipeline{
		Id:      uuid.Must(uuid.NewV7()),
		Name:    "mr",
		Version: "1",
		Blocks: []core.PipelineBlock{
			{Id: a, Name: "src", Inputs: nil, Args: map[string]any{}},
			{Id: m, Name: "mapper", Inputs: []core.InputRef{{ID: a}}, Args: map[string]any{}},
			{Id: mid, Name: "middle", Inputs: []core.InputRef{{ID: m}}, Args: map[string]any{}},
			{Id: red, Name: "reducer", Inputs: []core.InputRef{{ID: mid}}, Args: map[string]any{}},
		},
	}
	return p, []uuid.UUID{a, m, mid, red}
}

func TestMapReduceFanOutAndGather(t *testing.T) {
	eng, mem, pub, _, mp := newTestEngine(t)
	p, ids := mapReducePipeline(t, mp)
	ctx := context.Background()
	if err := eng.SubmitPipeline(ctx, &p, nil, ""); err != nil {
		t.Fatalf("SubmitPipeline: %v", err)
	}
	// 1) src dispatched.
	_ = eng.dispatchSweep(ctx)
	if got := len(pub.PublishedJobs()); got != 1 {
		t.Fatalf("after submit: expected 1 dispatch, got %d", got)
	}
	// 2) src completes; mapper goes out.
	if err := eng.applyResult(ctx, core.WorkerResult{
		InvocationID: ids[0].String(), PipelineID: p.Id, Status: core.ExecutionStatusComplete,
	}); err != nil {
		t.Fatal(err)
	}
	_ = eng.dispatchSweep(ctx)
	if got := len(pub.PublishedJobs()); got != 2 {
		t.Fatalf("after src complete: expected 2 dispatches, got %d", got)
	}
	if pub.PublishedJobs()[1].Assignment.BlockName != "mapper" {
		t.Errorf("expected mapper, got %s", pub.PublishedJobs()[1].Assignment.BlockName)
	}
	// 3) mapper returns 3-item expansion.
	exp := &core.ExpansionManifest{
		Items: []core.ExpansionItem{
			{Path: "inputs/source/a.tif", Key: "a"},
			{Path: "inputs/source/b.tif", Key: "b"},
			{Path: "inputs/source/c.tif", Key: "c"},
		},
	}
	if err := eng.applyResult(ctx, core.WorkerResult{
		InvocationID: ids[1].String(), PipelineID: p.Id, Status: core.ExecutionStatusMap, Expansion: exp,
	}); err != nil {
		t.Fatalf("mapper result: %v", err)
	}
	_ = eng.dispatchSweep(ctx)
	// 4) 3 middle invocations should now be dispatched.
	jobs := pub.PublishedJobs()
	if got := len(jobs); got != 5 {
		t.Fatalf("after expansion: expected 5 total dispatches, got %d (jobs=%v)", got, jobNames(jobs))
	}
	var midCount int
	for _, j := range jobs[2:] {
		if j.Assignment.BlockName == "middle" {
			midCount++
		}
	}
	if midCount != 3 {
		t.Fatalf("expected 3 middle dispatches, got %d", midCount)
	}
	// 5) Complete the three mapped invocations.  The scheduler keys
	//    fan-out completions under SHA1(blockID, "i"), but reports the
	//    invocation as "<UUID>.<i>".  Drive that through applyResult.
	for i := 0; i < 3; i++ {
		invID := ids[2].String() + "." + itoa(i)
		if err := eng.applyResult(ctx, core.WorkerResult{
			InvocationID: invID, PipelineID: p.Id, Status: core.ExecutionStatusComplete,
		}); err != nil {
			t.Fatalf("middle %d: %v", i, err)
		}
	}
	_ = eng.dispatchSweep(ctx)
	jobs = pub.PublishedJobs()
	// 6) Reducer must be dispatched exactly once after all three mids complete.
	var reducerCount int
	for _, j := range jobs {
		if j.Assignment.BlockName == "reducer" {
			reducerCount++
		}
	}
	if reducerCount != 1 {
		t.Fatalf("expected 1 reducer dispatch, got %d (jobs=%v)", reducerCount, jobNames(jobs))
	}
	// 7) Reducer completes; pipeline marked complete.
	if err := eng.applyResult(ctx, core.WorkerResult{
		InvocationID: ids[3].String(), PipelineID: p.Id, Status: core.ExecutionStatusComplete,
	}); err != nil {
		t.Fatal(err)
	}
	rec, _ := mem.LoadPipeline(ctx, p.Id)
	if rec.Status != store.PipelineComplete {
		t.Errorf("pipeline status: %s want complete", rec.Status)
	}
}

func jobNames(jobs []spade.Job) []string {
	out := make([]string, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, j.Assignment.BlockName+"["+j.Assignment.InvocationID+"]")
	}
	return out
}

// itoa is a tiny conversion helper; strconv.Itoa would do, but avoiding
// the extra import keeps the test file's imports compact.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

