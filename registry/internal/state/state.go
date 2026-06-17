// Package state implements the collection-version lifecycle state machine
// (registry.md §3) and its authorization rules (§11). It is the single place
// that mutates a version's state, and on every transition it also writes the
// audit log and drives the metadata mirror.
package state

import (
	"errors"
	"fmt"

	"spade_registry/internal/audit"
	"spade_registry/internal/mirror"
	"spade_registry/internal/store"
)

// Errors returned by the machine.
var (
	ErrIllegalTransition = errors.New("state: illegal transition")
	ErrUnauthorized      = errors.New("state: unauthorized")
)

// allowed encodes the legal transitions of registry.md §3. The build pipeline
// drives the happy path (submitted→…→available) and the failure edges; the
// "off" states are reached by owner/operator action. recalled is reachable from
// any state and is terminal.
var allowed = map[store.State][]store.State{
	store.StateSubmitted:  {store.StateScreening, store.StateFailed},
	store.StateScreening:  {store.StateScreened, store.StateFailed},
	store.StateScreened:   {store.StateBuilding, store.StateFailed},
	store.StateBuilding:   {store.StateAvailable, store.StateFailed},
	store.StateAvailable:  {store.StateDeprecated, store.StateYanked},
	store.StateDeprecated: {store.StateYanked},
	store.StateYanked:     {},
	store.StateFailed:     {},
	store.StateRecalled:   {}, // terminal
}

// CanTransition reports whether from→to is structurally legal. Any state may
// transition to recalled (the hard security switch, §3).
func CanTransition(from, to store.State) bool {
	if to == store.StateRecalled {
		return from != store.StateRecalled // irreversible; no self-loop
	}
	for _, t := range allowed[from] {
		if t == to {
			return true
		}
	}
	return false
}

// Actor identifies who is requesting a transition.
type Actor struct {
	ID         string
	Type       string // audit.ActorDeveloper | ActorOperator | ActorSystem
	IsOwner    bool   // owns the target collection
	IsOperator bool   // operator/admin privilege
}

// Authorize enforces §11: owners may deprecate/yank their collections;
// recall is operator-only; the build pipeline (system actor) drives the
// screening→building→available and →failed edges.
func Authorize(a Actor, to store.State) error {
	switch to {
	case store.StateRecalled:
		if a.IsOperator {
			return nil
		}
		return fmt.Errorf("%w: recall requires operator", ErrUnauthorized)
	case store.StateDeprecated, store.StateYanked:
		if a.IsOwner || a.IsOperator {
			return nil
		}
		return fmt.Errorf("%w: deprecate/yank requires collection owner", ErrUnauthorized)
	case store.StateScreening, store.StateScreened, store.StateBuilding,
		store.StateAvailable, store.StateFailed:
		if a.Type == audit.ActorSystem || a.IsOperator {
			return nil
		}
		return fmt.Errorf("%w: pipeline transition is system-driven", ErrUnauthorized)
	default:
		return fmt.Errorf("%w: unknown target state %q", ErrIllegalTransition, to)
	}
}

// Machine applies authorized, legal transitions and keeps audit + mirror in sync.
type Machine struct {
	st     *store.Store
	audit  *audit.Logger
	mirror mirror.Mirror
}

// New builds a Machine.
func New(st *store.Store, a *audit.Logger, m mirror.Mirror) *Machine {
	if m == nil {
		m = mirror.NoopMirror{}
	}
	return &Machine{st: st, audit: a, mirror: m}
}

// Transition validates and applies from→to for the given version.
//
// collectionName is used for audit + mirror; blocks are the version's block
// metadata (needed by the mirror on available/off transitions). errMsg is
// persisted when to == failed.
func (m *Machine) Transition(a Actor, v *store.Version, collectionName string, to store.State, reason, errMsg string) error {
	from := v.State
	if !CanTransition(from, to) {
		return fmt.Errorf("%w: %s→%s", ErrIllegalTransition, from, to)
	}
	if err := Authorize(a, to); err != nil {
		return err
	}
	if err := m.st.SetVersionState(v.ID, to, errMsg); err != nil {
		return err
	}
	v.State = to

	// Audit every transition (best-effort logging must not fail the change).
	_ = m.audit.Transition(a.ID, a.Type, collectionName, v.Version, from, to, reason)

	// Drive the browse mirror. Failures are logged, never fatal (§10).
	m.applyMirror(v, collectionName, to)
	return nil
}

func (m *Machine) applyMirror(v *store.Version, collectionName string, to store.State) {
	blocks, err := m.st.ListBlockMeta(v.ID)
	if err != nil {
		return
	}
	switch to {
	case store.StateAvailable:
		_ = m.mirror.UpsertVersion(v, collectionName, blocks)
	case store.StateDeprecated, store.StateYanked, store.StateRecalled:
		// Deprecated hides from browse; yanked blocks new installs; recalled is
		// the hard switch — all three are removed from the browse mirror.
		_ = m.mirror.RemoveVersion(blocks)
	}
}
