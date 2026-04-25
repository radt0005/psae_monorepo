import type { Node, Edge } from "@vue-flow/core";

/**
 * Pipeline & block types per spec/pipeline.md and spec/blocks.md.
 *
 * The previous prototype used Block.input (singular) and a separate
 * Pipeline.data array. Both are gone; the spec has a single `blocks: Block[]`
 * with `inputs: InputRef[]`.
 */

export type ArgKeyValueStore = Record<string, any>;

/** Reference to another block in the same pipeline. */
export type InputRef =
  | string // bare invocation id (type-matched at the worker)
  | { block: string; output: string }; // explicit named-output mapping

export type Block = {
  id: string;
  name: string;
  inputs: InputRef[];
  args: ArgKeyValueStore;
};

export type Pipeline = {
  id: string;
  name: string;
  version: string;
  description?: string;
  blocks: Block[];
};

export type PipelineMetadata = {
  id: string;
  name: string;
  version: string;
  description?: string;
};

/* -------------------------- block manifests --------------------------- */

export type PortType =
  | "file"
  | "directory"
  | "collection"
  | "json"
  | "expansion"
  | "string"
  | "number"
  | "boolean";

export type PortDecl = {
  type: PortType;
  format?: string;
  description?: string;
  /** For type === "collection". */
  item_type?: PortType;
};

export type BlockKind = "standard" | "map" | "reduce";

/** spec/blocks.md §3 manifest. */
export type BlockManifest = {
  id: string;
  version: string;
  kind: BlockKind;
  network: boolean;
  description?: string;
  entrypoint?: string;
  inputs: Record<string, PortDecl>;
  outputs: Record<string, PortDecl>;
};

/** Lightweight summary used in lists. */
export type BlockListItem = {
  id: string;
  name: string;
  label: string;
  kind: BlockKind;
  network: boolean;
  description?: string;
};

export type BlockList = { blocks: BlockListItem[] };

/* -------------------------- helpers ----------------------------------- */

export type NodesAndEdges = { nodes: Node[]; edges: Edge[] };

export type Run = {
  runId: string;
  userId: string;
  content: string;
  status: "pending" | "running" | "error" | "completed";
};

/** Edge data carried by Vue Flow edges; bindings are produced by the wiring resolver. */
export type EdgeBindings = Array<{ output: string; input: string }>;

export type EdgeData = {
  /** Resolved bindings between upstream outputs and downstream inputs. */
  bindings?: EdgeBindings;
  /** True until the user has resolved an ambiguous mapping. */
  unresolved?: boolean;
};
