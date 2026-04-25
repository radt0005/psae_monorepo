import { describe, it, expect } from "vitest";
import { PipelineBuilder } from "~/utils/pipeline";
import type { Block, Pipeline } from "~/utils/types";

const node = (id: string, name: string, args: Record<string, any> = {}) => ({
  id,
  type: "block",
  position: { x: 0, y: 0 },
  data: { label: name, name, args },
});

const edge = (
  source: string,
  target: string,
  bindings?: Array<{ output: string; input: string }>,
) => ({
  id: `${source}->${target}`,
  source,
  target,
  data: bindings ? { bindings, unresolved: false } : undefined,
});

describe("PipelineBuilder", () => {
  it("emits a spec-conformant Pipeline with `inputs` plural", () => {
    const b = new PipelineBuilder({ name: "test", version: "1.0" });
    b.addBlockFromNode(node("a", "data.source"));
    b.addBlockFromNode(node("b", "raster.process"));
    b.parseEdge(edge("a", "b"));

    const out = b.exportPipeline();

    expect(out.name).toBe("test");
    expect(out.version).toBe("1.0");
    expect(out.blocks).toHaveLength(2);
    expect(out.blocks[1].inputs).toEqual(["a"]);
    expect((out as any).data).toBeUndefined();
    // bare reference is a string
    expect(typeof out.blocks[1].inputs[0]).toBe("string");
  });

  it("emits explicit {block, output} references when edges carry bindings", () => {
    const b = new PipelineBuilder();
    b.addBlockFromNode(node("a", "raster.split"));
    b.addBlockFromNode(node("b", "raster.process"));
    b.parseEdge(edge("a", "b", [{ output: "tiles", input: "source" }]));

    const out = b.exportPipeline();
    expect(out.blocks[1].inputs).toEqual([{ block: "a", output: "tiles" }]);
  });

  it("dedupes duplicate bare and explicit refs", () => {
    const b = new PipelineBuilder();
    b.addBlockFromNode(node("a", "x"));
    b.addBlockFromNode(node("b", "y"));
    // simulate two edges added between same pair (shouldn't happen, but be safe)
    b.parseEdge(edge("a", "b"));
    b.parseEdge(edge("a", "b"));
    expect(b.exportPipeline().blocks[1].inputs).toEqual(["a"]);
  });

  it("freshSubmissionId() rotates the pipeline id (per pipeline.md §10)", () => {
    const b = new PipelineBuilder();
    const first = b.exportPipeline().id;
    b.freshSubmissionId();
    const second = b.exportPipeline().id;
    expect(second).not.toBe(first);
  });

  it("respects empty inputs / args", () => {
    const b = new PipelineBuilder();
    b.addBlockFromNode(node("a", "source"));
    const out = b.exportPipeline();
    expect(out.blocks[0].inputs).toEqual([]);
    expect(out.blocks[0].args).toEqual({});
  });
});

describe("Block / Pipeline types", () => {
  it("Block.inputs is plural and Pipeline has no `data`", () => {
    const block: Block = { id: "x", name: "n", inputs: [], args: {} };
    const pipeline: Pipeline = {
      id: "p",
      name: "n",
      version: "1.0",
      blocks: [block],
    };
    // type-level: this would not compile if `inputs` were renamed back to
    // `input` or if `data` were re-added as required. We check shape too.
    expect(pipeline.blocks[0].inputs).toEqual([]);
    expect((pipeline as any).data).toBeUndefined();
  });
});
