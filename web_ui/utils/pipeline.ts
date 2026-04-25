import type { Node, Edge } from "@vue-flow/core";
import { v7 } from "uuid";
import type {
  Pipeline,
  Block,
  PipelineMetadata,
  InputRef,
  EdgeData,
} from "./types";

/**
 * Build a spec/pipeline.md compliant Pipeline from the editor's node + edge state.
 *
 * Reference resolution rules (mirrors pipeline.md §5):
 *   - Edges that carry resolved bindings on `edge.data` produce explicit
 *     {block, output} references — one per binding.
 *   - Edges without bindings produce a bare invocation id (type matching
 *     happens at the worker / `spade check`).
 */
export class PipelineBuilder {
  blocks: Block[] = [];
  metadata: PipelineMetadata;

  constructor(metadata?: Partial<PipelineMetadata>) {
    this.metadata = {
      id: metadata?.id ?? v7(),
      name: metadata?.name ?? "untitled-pipeline",
      version: metadata?.version ?? "0.1.0",
      description: metadata?.description,
    };
  }

  addBlockFromNode(node: Node) {
    this.blocks.push({
      id: node.id,
      name: node.data?.name ?? node.data?.label ?? "",
      inputs: [],
      args: node.data?.args ?? {},
    });
  }

  parseEdge(edge: Edge) {
    const upstream = edge.source;
    const downstream = edge.target;
    const target = this.blocks.find((b) => b.id === downstream);
    if (!target) return;

    const data = edge.data as EdgeData | undefined;
    if (data?.bindings && data.bindings.length > 0) {
      for (const binding of data.bindings) {
        target.inputs.push({ block: upstream, output: binding.output });
      }
    } else {
      target.inputs.push(upstream);
    }
  }

  setName(name: string) {
    this.metadata.name = name;
  }

  setDescription(desc: string) {
    this.metadata.description = desc;
  }

  setVersion(version: string) {
    this.metadata.version = version;
  }

  /** Generate a fresh pipeline id (use at submission time per pipeline.md §10). */
  freshSubmissionId() {
    this.metadata.id = v7();
  }

  exportPipeline(): Pipeline {
    return {
      id: this.metadata.id,
      name: this.metadata.name,
      version: this.metadata.version,
      description: this.metadata.description,
      blocks: this.blocks.map((b) => ({
        id: b.id,
        name: b.name,
        inputs: dedupeInputs(b.inputs),
        args: b.args ?? {},
      })),
    };
  }
}

/** Drop duplicate bare references (a block referenced twice as a bare id). */
function dedupeInputs(inputs: InputRef[]): InputRef[] {
  const seenBare = new Set<string>();
  const seenExplicit = new Set<string>();
  const out: InputRef[] = [];
  for (const ref of inputs) {
    if (typeof ref === "string") {
      if (seenBare.has(ref)) continue;
      seenBare.add(ref);
      out.push(ref);
    } else {
      const key = `${ref.block}::${ref.output}`;
      if (seenExplicit.has(key)) continue;
      seenExplicit.add(key);
      out.push(ref);
    }
  }
  return out;
}
