// Package audit writes the registry's append-only audit log: publish requests,
// state transitions, and worker fetches (registry.md §7.3).
package audit

import (
	"encoding/json"

	"spade_registry/internal/store"
)

// Actor types.
const (
	ActorDeveloper = "developer"
	ActorWorker    = "worker"
	ActorOperator  = "operator"
	ActorSystem    = "system"
)

// Logger appends audit entries.
type Logger struct {
	st *store.Store
}

// New wraps a store.
func New(st *store.Store) *Logger { return &Logger{st: st} }

// Publish records a publish request.
func (l *Logger) Publish(actorID, collection, version, repoURL, sha string) error {
	detail, _ := json.Marshal(map[string]string{"repo_url": repoURL, "commit_sha": sha})
	return l.st.CreateAuditEntry(&store.AuditEntry{
		EventType:  "publish",
		ActorID:    actorID,
		ActorType:  ActorDeveloper,
		Collection: collection,
		Version:    version,
		Detail:     detail,
	})
}

// Transition records a lifecycle state change.
func (l *Logger) Transition(actorID, actorType, collection, version string, from, to store.State, reason string) error {
	return l.st.CreateAuditEntry(&store.AuditEntry{
		EventType:  "transition",
		ActorID:    actorID,
		ActorType:  actorType,
		Collection: collection,
		Version:    version,
		FromState:  string(from),
		ToState:    string(to),
		Reason:     reason,
	})
}

// Fetch records a worker artifact download (for incident response).
func (l *Logger) Fetch(workerID, collection, version, platform, arch string) error {
	detail, _ := json.Marshal(map[string]string{"platform": platform, "arch": arch})
	return l.st.CreateAuditEntry(&store.AuditEntry{
		EventType:  "fetch",
		ActorID:    workerID,
		ActorType:  ActorWorker,
		Collection: collection,
		Version:    version,
		Detail:     detail,
	})
}
