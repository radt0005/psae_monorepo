package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// --- Phase 1.1: Input Reference Types ---

// InputRef supports both forms of pipeline input references:
// bare invocation IDs and explicit block+output pairs.  When a block has
// multiple same-typed inputs the explicit form can also set "as" to name
// the downstream input that should receive this reference.
type InputRef struct {
	Block  *uuid.UUID `yaml:"block,omitempty"`  // nil for bare references
	Output string     `yaml:"output,omitempty"` // named output on the dependency
	As     string     `yaml:"as,omitempty"`     // optional downstream input name
	ID     uuid.UUID  `yaml:"-"`                // populated for bare references
}

func (r *InputRef) UnmarshalYAML(value *yaml.Node) error {
	// Bare reference: a plain UUID string
	if value.Kind == yaml.ScalarNode {
		id, err := uuid.Parse(value.Value)
		if err != nil {
			return fmt.Errorf("invalid bare input reference UUID %q: %w", value.Value, err)
		}
		r.ID = id
		r.Block = nil
		r.Output = ""
		return nil
	}

	// Explicit reference: a mapping with block and output keys
	if value.Kind == yaml.MappingNode {
		var m struct {
			Block  uuid.UUID `yaml:"block"`
			Output string    `yaml:"output"`
			As     string    `yaml:"as"`
		}
		if err := value.Decode(&m); err != nil {
			return fmt.Errorf("invalid explicit input reference: %w", err)
		}
		r.Block = &m.Block
		r.Output = m.Output
		r.As = m.As
		r.ID = uuid.Nil
		return nil
	}

	return fmt.Errorf("input reference must be a UUID string or a mapping with block and output keys")
}

func (r InputRef) MarshalYAML() (any, error) {
	if r.Block == nil {
		// Bare reference: serialize as plain UUID string
		return r.ID.String(), nil
	}
	// Explicit reference: serialize as mapping
	m := struct {
		Block  uuid.UUID `yaml:"block"`
		Output string    `yaml:"output"`
		As     string    `yaml:"as,omitempty"`
	}{
		Block:  *r.Block,
		Output: r.Output,
		As:     r.As,
	}
	return m, nil
}

// --- Phase 1.2: Block Manifest Types ---

// BlockKind represents the kind of block (standard, map, or reduce).
type BlockKind string

const (
	BlockKindStandard BlockKind = "standard"
	BlockKindMap      BlockKind = "map"
	BlockKindReduce   BlockKind = "reduce"
)

// InputDeclaration describes a declared input in a block manifest.
type InputDeclaration struct {
	Type        string `yaml:"type" json:"type"`
	Format      string `yaml:"format,omitempty" json:"format,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	ItemType    string `yaml:"item_type,omitempty" json:"item_type,omitempty"`
}

// OutputDeclaration describes a declared output in a block manifest.
type OutputDeclaration struct {
	Type        string `yaml:"type" json:"type"`
	Format      string `yaml:"format,omitempty" json:"format,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	ItemType    string `yaml:"item_type,omitempty" json:"item_type,omitempty"`
}

// BlockManifest represents a parsed block.yaml file.
type BlockManifest struct {
	ID          string                       `yaml:"id" json:"id"`
	Version     string                       `yaml:"version" json:"version"`
	Kind        BlockKind                    `yaml:"kind,omitempty" json:"kind,omitempty"`
	Network     bool                         `yaml:"network,omitempty" json:"network,omitempty"`
	Description string                       `yaml:"description,omitempty" json:"description,omitempty"`
	Entrypoint  string                       `yaml:"entrypoint,omitempty" json:"entrypoint,omitempty"`
	Inputs      map[string]InputDeclaration  `yaml:"inputs" json:"inputs"`
	Outputs     map[string]OutputDeclaration `yaml:"outputs" json:"outputs"`
}

// LoadBlockManifest reads and parses a block.yaml file into a BlockManifest.
func LoadBlockManifest(path string) (BlockManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BlockManifest{}, fmt.Errorf("reading block manifest %s: %w", path, err)
	}
	var m BlockManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return BlockManifest{}, fmt.Errorf("parsing block manifest %s: %w", path, err)
	}
	if m.Kind == "" {
		m.Kind = BlockKindStandard
	}
	return m, nil
}

// --- Phase 1.3: Pipeline Types ---

// PipelineBlock represents a block invocation within a pipeline definition.
type PipelineBlock struct {
	Id   uuid.UUID `yaml:"id"`
	Name string    `yaml:"name"`
	// Version pins the collection artifact version this block resolves to
	// (Option A, INSTALLER_IMPLEMENTATION_PLAN §Phase 3). The web editor sets it
	// from the metadata mirror at authoring time. Empty ⇒ legacy lookup (latest
	// installed, no registry fetch).
	Version string         `yaml:"version,omitempty"`
	Inputs  []InputRef     `yaml:"inputs"`
	Outputs []string       `yaml:"outputs,omitempty"`
	Args    map[string]any `yaml:"args"`
	// Secrets maps a block's logical secret name (the get_secret argument) to
	// the user's stored secret name. Values are names, never secret values —
	// the values are resolved at dispatch (cloud) or from the local keychain
	// (local) and injected into the sandbox. See spec/secrets.md §3.2.
	Secrets map[string]string `yaml:"secrets,omitempty"`
}

// Pipeline represents a complete pipeline definition.
type Pipeline struct {
	Id          uuid.UUID       `yaml:"id"`
	Name        string          `yaml:"name"`
	Version     string          `yaml:"version"`
	Description string          `yaml:"description,omitempty"`
	Blocks      []PipelineBlock `yaml:"blocks"`
}

// LoadPipeline reads and parses a pipeline YAML file.
func LoadPipeline(path string) (Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Pipeline{}, fmt.Errorf("reading pipeline %s: %w", path, err)
	}
	var p Pipeline
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Pipeline{}, fmt.Errorf("parsing pipeline %s: %w", path, err)
	}
	return p, nil
}

// SavePipeline serializes a Pipeline to YAML.
func SavePipeline(pipeline Pipeline, path string) error {
	data, err := yaml.Marshal(&pipeline)
	if err != nil {
		return fmt.Errorf("marshaling pipeline: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory for pipeline: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing pipeline %s: %w", path, err)
	}
	return nil
}

// --- Phase 1.4: Execution and Invocation Types ---

// BlockInvocation represents a single block execution.
type BlockInvocation struct {
	Id         uuid.UUID      `yaml:"id"`
	BlockId    string         `yaml:"block_id"`
	PipelineId uuid.UUID      `yaml:"pipeline_id"`
	Inputs     []InputRef     `yaml:"inputs"`
	Arguments  map[string]any `yaml:"arguments"`
	// MapIndices is the invocation's index vector: one integer per
	// enclosing map context, outermost first.  nil/empty means the block
	// is not mapped.  A block nested two maps deep has len == 2.
	MapIndices []int `yaml:"map_indices,omitempty"`
}

// InvocationID returns the invocation ID string: the block UUID followed by
// one ".<index>" component per enclosing map context, outermost first
// (e.g. "0197...-edc", "0197...-edc.3", "0197...-edc.3.0").
func (b *BlockInvocation) InvocationID() string {
	return FormatInvocationID(b.Id, b.MapIndices)
}

// MapDepth returns the number of enclosing map contexts this invocation
// runs under (0 for a non-mapped block).
func (b *BlockInvocation) MapDepth() int {
	return len(b.MapIndices)
}

// FormatInvocationID renders a block UUID plus index vector as the dotted
// invocation ID string used for work directories, object-storage keys, and
// scheduler bookkeeping.
func FormatInvocationID(id uuid.UUID, indices []int) string {
	var sb strings.Builder
	sb.WriteString(id.String())
	for _, i := range indices {
		sb.WriteString(fmt.Sprintf(".%d", i))
	}
	return sb.String()
}

// IndexPrefix renders the first d components of an index vector as a dotted
// string ("" for d == 0, "3" for one component, "3.0" for two).  It is the
// key used for per-instance map-context bookkeeping.
func IndexPrefix(indices []int, d int) string {
	if d > len(indices) {
		d = len(indices)
	}
	var sb strings.Builder
	for i := 0; i < d; i++ {
		if i > 0 {
			sb.WriteByte('.')
		}
		sb.WriteString(strconv.Itoa(indices[i]))
	}
	return sb.String()
}

// ParseInvocationID splits an invocation ID of the form
// "<uuid>[.<i>[.<j>…]]" into the block UUID and its index vector.  A bare
// UUID returns a nil vector.
func ParseInvocationID(id string) (uuid.UUID, []int, error) {
	raw := id
	var indices []int
	for {
		dot := strings.LastIndex(raw, ".")
		if dot < 0 {
			break
		}
		n, err := strconv.Atoi(raw[dot+1:])
		if err != nil || n < 0 {
			break // suffix is not a non-negative integer — part of the UUID
		}
		indices = append([]int{n}, indices...)
		raw = raw[:dot]
	}
	u, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("parsing invocation id %q: %w", id, err)
	}
	return u, indices, nil
}

// ExecutionStatus represents the current status of a block execution.
type ExecutionStatus string

const (
	ExecutionStatusAwaiting  ExecutionStatus = "waiting"
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusComplete  ExecutionStatus = "complete"
	ExecutionStatusError     ExecutionStatus = "error"
	ExecutionStatusMap       ExecutionStatus = "map"
	ExecutionStatusReduce    ExecutionStatus = "reduce"
)

// BlockInvocationResult represents the result of executing a block.
type BlockInvocationResult struct {
	Id         uuid.UUID `yaml:"id"`
	PipelineId uuid.UUID `yaml:"pipeline_id"`
	// MapIndices is the index vector of the invocation that produced this
	// result (nil for non-mapped blocks).  Together with Id it identifies
	// the invocation: see InvocationID().
	MapIndices []int              `yaml:"map_indices,omitempty"`
	Status     ExecutionStatus    `yaml:"status"`
	Outputs    []string           `yaml:"outputs"`
	Expansion  *ExpansionManifest `yaml:"expansion,omitempty"`
	Error      string             `yaml:"error,omitempty"`
	// ExitCode is the subprocess exit code. Zero on success, non-zero
	// when the block process itself returned a failure.  Set to -1 when
	// the block never ran (setup error, hash mismatch, etc.).
	ExitCode int    `yaml:"exit_code"`
	LogsPath string `yaml:"logs_path,omitempty"`
}

// InvocationID returns the invocation ID string of the invocation that
// produced this result (see BlockInvocation.InvocationID).
func (r *BlockInvocationResult) InvocationID() string {
	return FormatInvocationID(r.Id, r.MapIndices)
}

// Worker represents a worker node.
type Worker struct {
	Id          uuid.UUID
	Ip          string
	Description string
}

// --- Phase 1.5: Expansion Manifest Types ---

// ExpansionItem represents a single item in an expansion manifest.
type ExpansionItem struct {
	Path string `yaml:"path"`
	Key  string `yaml:"key"`
}

// ExpansionManifest is the parsed form of an expansion.yaml file written by map blocks.
type ExpansionManifest struct {
	Items []ExpansionItem `yaml:"items"`
}

// LoadExpansionManifest reads and parses an expansion.yaml file.
func LoadExpansionManifest(path string) (ExpansionManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExpansionManifest{}, fmt.Errorf("reading expansion manifest %s: %w", path, err)
	}
	var m ExpansionManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return ExpansionManifest{}, fmt.Errorf("parsing expansion manifest %s: %w", path, err)
	}
	return m, nil
}

// --- Phase 1.6: Invocation Metadata Type ---

// InvocationMetadataInput describes a single input entry in invocation.yaml.
type InvocationMetadataInput struct {
	Path string `yaml:"path"`
	Hash string `yaml:"hash"`
}

// InvocationMetadataBlock describes the block identity in invocation.yaml.
type InvocationMetadataBlock struct {
	ID      string `yaml:"id"`
	Version string `yaml:"version"`
}

// InvocationMetadata matches the invocation.yaml format from blocks.md section 7.
type InvocationMetadata struct {
	Block        InvocationMetadataBlock            `yaml:"block"`
	InvocationID string                             `yaml:"invocation_id"`
	Inputs       map[string]InvocationMetadataInput `yaml:"inputs"`
}

// WriteInvocationMetadata serializes the metadata to invocation.yaml in the given directory.
func WriteInvocationMetadata(meta InvocationMetadata, dir string) error {
	data, err := yaml.Marshal(&meta)
	if err != nil {
		return fmt.Errorf("marshaling invocation metadata: %w", err)
	}
	path := filepath.Join(dir, "invocation.yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing invocation metadata: %w", err)
	}
	return nil
}

// --- Phase 1.7: Block Registry Types ---

// BlockRegistryEntry represents a block in the SQLite registry with GORM model tags.
type BlockRegistryEntry struct {
	ID                uint   `gorm:"primaryKey"`
	CollectionName    string `gorm:"index"`
	CollectionVersion string
	BlockName         string `gorm:"index"`
	BlockID           string `gorm:"index"` // e.g. "gdal.rasterize"
	Language          string
	Entrypoint        string
	InstalledPath     string
	ContentHash       string
	Kind              string
	Network           bool
	ManifestJSON      string // serialized block manifest

	// Install provenance and recall bookkeeping (worker.md §Block Index). These
	// are empty for locally-built (seed-blocks / `spade install`) rows, which
	// leaves Source="" (treated as local): the registry freshness/recall path
	// engages only for Source=InstallSourceRegistry entries.
	Source         string    // InstallSourceRegistry | InstallSourceLocal
	Signature      string    // base64 ed25519 signature over the artifact (registry installs)
	RegistryState  string    // last-seen registry state: available|deprecated|yanked|recalled
	LastVerifiedAt time.Time // when RegistryState was last confirmed with the registry
}

// Install sources recorded in BlockRegistryEntry.Source.
const (
	InstallSourceRegistry = "registry"
	InstallSourceLocal    = "local"
)

// blockNameFromID returns the short block name from a fully-qualified
// manifest ID.  Block IDs follow the convention "<collection>.<block>"
// (e.g. "data.read"); registry entries and block-dispatcher subcommands
// want just the trailing "<block>" segment.
func blockNameFromID(id string) string {
	if i := strings.LastIndex(id, "."); i >= 0 {
		return id[i+1:]
	}
	return id
}

// CollectionNameFromBlockID returns the collection segment of a block ID
// ("<collection>.<block>" → "<collection>"). The worker uses it with the pinned
// CollectionVersion to form the registry fetch key.
func CollectionNameFromBlockID(id string) string {
	if i := strings.Index(id, "."); i >= 0 {
		return id[:i]
	}
	return id
}

// CollectionLanguage represents the detected language of a block collection.
type CollectionLanguage string

const (
	CollectionLanguageRust       CollectionLanguage = "rust"
	CollectionLanguageGo         CollectionLanguage = "go"
	CollectionLanguagePython     CollectionLanguage = "python"
	CollectionLanguageTypeScript CollectionLanguage = "typescript"
	CollectionLanguageR          CollectionLanguage = "r"
)

// --- Phase 1.8: Worker Communication Types ---

// WorkerAssignment represents a block execution assignment sent from scheduler to worker.
type WorkerAssignment struct {
	InvocationID string `json:"invocation_id"`
	BlockName    string `json:"block_name"`
	// CollectionVersion pins the collection artifact version the worker must have
	// installed to run this block (Option A). It carries the pin from
	// PipelineBlock.Version to the worker's registry-fetch key. Empty ⇒ legacy
	// behavior (LookupBlock latest, no fetch), preserving seed-blocks + old tests.
	CollectionVersion string         `json:"collection_version,omitempty"`
	PipelineID        uuid.UUID      `json:"pipeline_id"`
	WorkDir           string         `json:"work_dir"`
	Args              map[string]any `json:"args"`
	Inputs            []InputRef     `json:"inputs"`
}

// WorkerResult represents the completion response from worker to scheduler.
type WorkerResult struct {
	InvocationID string             `json:"invocation_id"`
	PipelineID   uuid.UUID          `json:"pipeline_id"`
	Status       ExecutionStatus    `json:"status"`
	Error        string             `json:"error,omitempty"`
	Expansion    *ExpansionManifest `json:"expansion,omitempty"`
	OutputHashes map[string]string  `json:"output_hashes,omitempty"`
	// ExitCode is the subprocess exit code.  Zero on success, non-zero on
	// block failure.  -1 when the block never ran.
	ExitCode int `json:"exit_code"`
	// LogsPath is the absolute path to the invocation's logs directory
	// on the shared worker filesystem, so the scheduler / UI can fetch
	// stdout.log and stderr.log for debugging.
	LogsPath string `json:"logs_path,omitempty"`
}
