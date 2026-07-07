package engine

import (
	"context"
	"crypto/ed25519"
	"log/slog"
	"testing"
	"time"

	"captoken"
	"core"

	"github.com/google/uuid"
)

func newSecretsTestEngine(t *testing.T) (*Engine, ed25519.PublicKey) {
	t.Helper()
	pub, priv, _ := captoken.GenerateKey()
	e := &Engine{logger: slog.Default(), ownerByPipeline: map[uuid.UUID]string{}}
	e.SetTokenSigner(captoken.NewSigner(priv), time.Minute)
	return e, pub
}

func TestMintCapabilityToken(t *testing.T) {
	e, pub := newSecretsTestEngine(t)

	pid, bid := uuid.New(), uuid.New()
	e.ownerByPipeline[pid] = "alice" // seed owner so the store isn't consulted
	pipeline := core.Pipeline{Id: pid, Blocks: []core.PipelineBlock{{
		Id: bid, Name: "db.query",
		Secrets: map[string]string{"db": "prod-dsn"},
	}}}
	inv := core.BlockInvocation{Id: bid, PipelineId: pid, BlockId: "db.query"}

	tok := e.mintCapabilityToken(context.Background(), pipeline, inv)
	if tok == "" {
		t.Fatal("expected a capability token for a block that declares secrets")
	}
	claims, err := captoken.Verify(tok, []ed25519.PublicKey{pub})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.UserID != "alice" || claims.InvocationID != bid.String() {
		t.Fatalf("claims = %+v", claims)
	}
	// The token scopes the STORED name (the map value), not the logical name.
	if len(claims.SecretNames) != 1 || claims.SecretNames[0] != "prod-dsn" {
		t.Fatalf("scoped names = %v, want [prod-dsn]", claims.SecretNames)
	}
}

func TestMintNoSecretsYieldsNoToken(t *testing.T) {
	e, _ := newSecretsTestEngine(t)
	pid, bid := uuid.New(), uuid.New()
	e.ownerByPipeline[pid] = "alice"
	pipeline := core.Pipeline{Id: pid, Blocks: []core.PipelineBlock{{Id: bid, Name: "x"}}}
	inv := core.BlockInvocation{Id: bid, PipelineId: pid}
	if tok := e.mintCapabilityToken(context.Background(), pipeline, inv); tok != "" {
		t.Fatal("a block with no secrets should carry no token")
	}
}

func TestMintDisabledWithoutSigner(t *testing.T) {
	e := &Engine{logger: slog.Default(), ownerByPipeline: map[uuid.UUID]string{}}
	pid, bid := uuid.New(), uuid.New()
	pipeline := core.Pipeline{Id: pid, Blocks: []core.PipelineBlock{{
		Id: bid, Secrets: map[string]string{"db": "x"},
	}}}
	inv := core.BlockInvocation{Id: bid, PipelineId: pid}
	if tok := e.mintCapabilityToken(context.Background(), pipeline, inv); tok != "" {
		t.Fatal("no signer configured should yield no token")
	}
}
