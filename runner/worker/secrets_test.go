package worker

import (
	"context"
	"errors"
	"testing"

	"core"
	spade "spade_runner"

	"github.com/google/uuid"
)

// fakeResolver records the names it was asked for and returns canned values.
type fakeResolver struct {
	values    map[string]string
	err       error
	gotToken  string
	gotNames  []string
	callCount int
}

func (f *fakeResolver) Resolve(_ context.Context, token string, names []string) (map[string]string, error) {
	f.callCount++
	f.gotToken = token
	f.gotNames = names
	if f.err != nil {
		return nil, f.err
	}
	return f.values, nil
}

func jobWithBlock(pb core.PipelineBlock, tokenStr string) spade.Job {
	return spade.Job{
		Pipeline:        core.Pipeline{Blocks: []core.PipelineBlock{pb}},
		CapabilityToken: tokenStr,
	}
}

func TestResolveSecretsReKeysLogicalNames(t *testing.T) {
	res := &fakeResolver{values: map[string]string{"prod-dsn": "postgres://x", "ai-key": "sk-123"}}
	w := New(nil, t.TempDir(), WithSecretResolver(res))

	pb := core.PipelineBlock{
		Id:   uuid.New(),
		Name: "db.query",
		Secrets: map[string]string{
			"db":  "prod-dsn",
			"api": "ai-key",
		},
	}
	got, err := w.resolveSecrets(context.Background(), jobWithBlock(pb, "tok-abc"), pb)
	if err != nil {
		t.Fatalf("resolveSecrets: %v", err)
	}
	// Values are keyed by the block's LOGICAL names, not the stored names.
	if got["db"] != "postgres://x" || got["api"] != "sk-123" {
		t.Fatalf("re-keyed map = %v", got)
	}
	// The resolver was called with the capability token and the STORED names.
	if res.gotToken != "tok-abc" {
		t.Fatalf("token = %q", res.gotToken)
	}
	if len(res.gotNames) != 2 || res.gotNames[0] != "ai-key" || res.gotNames[1] != "prod-dsn" {
		t.Fatalf("requested names = %v, want sorted stored names", res.gotNames)
	}
}

func TestResolveSecretsNoneDeclared(t *testing.T) {
	res := &fakeResolver{}
	w := New(nil, t.TempDir(), WithSecretResolver(res))
	pb := core.PipelineBlock{Id: uuid.New(), Name: "x"}
	got, err := w.resolveSecrets(context.Background(), jobWithBlock(pb, ""), pb)
	if err != nil || got != nil {
		t.Fatalf("no secrets should yield (nil, nil), got (%v, %v)", got, err)
	}
	if res.callCount != 0 {
		t.Fatal("resolver should not be called when no secrets are declared")
	}
}

func TestResolveSecretsNoResolverIsWorkerError(t *testing.T) {
	w := New(nil, t.TempDir()) // no resolver configured
	pb := core.PipelineBlock{Id: uuid.New(), Name: "db.query", Secrets: map[string]string{"db": "prod-dsn"}}
	if _, err := w.resolveSecrets(context.Background(), jobWithBlock(pb, "t"), pb); err == nil {
		t.Fatal("a block that declares secrets with no resolver must be a worker-side error")
	}
}

func TestResolveSecretsKMSErrorPropagates(t *testing.T) {
	res := &fakeResolver{err: errors.New("boom")}
	w := New(nil, t.TempDir(), WithSecretResolver(res))
	pb := core.PipelineBlock{Id: uuid.New(), Name: "db.query", Secrets: map[string]string{"db": "prod-dsn"}}
	if _, err := w.resolveSecrets(context.Background(), jobWithBlock(pb, "t"), pb); err == nil {
		t.Fatal("a KMS resolve failure must propagate as a worker-side error")
	}
}

func TestResolveSecretsMissingNameIsError(t *testing.T) {
	// The KMS returns fewer names than requested.
	res := &fakeResolver{values: map[string]string{}}
	w := New(nil, t.TempDir(), WithSecretResolver(res))
	pb := core.PipelineBlock{Id: uuid.New(), Name: "db.query", Secrets: map[string]string{"db": "prod-dsn"}}
	if _, err := w.resolveSecrets(context.Background(), jobWithBlock(pb, "t"), pb); err == nil {
		t.Fatal("a stored name absent from the KMS response must be an error")
	}
}
