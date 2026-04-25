import { describe, it, expect, beforeEach } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import YAML from "yaml";
import { useFlow } from "~/composables/useFlow";

const PIPELINE_YAML = `
id: 019cf4bc-0000-7000-0000-000000000000
name: land-cover
version: "1.0"
description: classify land cover
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLY"
  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
    args:
      target_crs: "EPSG:4326"
  - id: 019cf4bc-3333-7000-0000-000000000000
    name: raster.clip
    inputs:
      - block: 019cf4bc-2222-7000-0000-000000000000
        output: clipped_raster
    args:
      boundary: "aoi.geojson"
`;

describe("useFlow store", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it("addNode mints UUIDv7 ids by default", () => {
    const flow = useFlow();
    const id = flow.addNode({ name: "x", label: "X" });
    // UUIDv7: 8-4-4-4-12 hex with 7 in the 13th nibble
    expect(id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}$/,
    );
    expect(flow.nodes.length).toBe(1);
  });

  it("removeNode also removes incident edges", () => {
    const flow = useFlow();
    const a = flow.addNode({ name: "a", label: "A" });
    const b = flow.addNode({ name: "b", label: "B" });
    flow.connect(a, b);
    expect(flow.edges.length).toBe(1);
    flow.removeNode(a);
    expect(flow.edges.length).toBe(0);
  });

  it("connect dedupes between the same pair", () => {
    const flow = useFlow();
    const a = flow.addNode({ name: "a", label: "A" });
    const b = flow.addNode({ name: "b", label: "B" });
    flow.connect(a, b);
    flow.connect(a, b);
    expect(flow.edges.length).toBe(1);
  });

  it("yamlToNodes preserves block ids verbatim (cache contract)", () => {
    const flow = useFlow();
    flow.yamlToNodes(PIPELINE_YAML);
    const ids = flow.nodes.map((n) => n.id);
    expect(ids).toContain("019cf4bc-1111-7000-0000-000000000000");
    expect(ids).toContain("019cf4bc-2222-7000-0000-000000000000");
    expect(ids).toContain("019cf4bc-3333-7000-0000-000000000000");
  });

  it("yamlToNodes preserves explicit {block, output} bindings on edges", () => {
    const flow = useFlow();
    flow.yamlToNodes(PIPELINE_YAML);
    const edge = flow.edges.find(
      (e) =>
        e.source === "019cf4bc-2222-7000-0000-000000000000" &&
        e.target === "019cf4bc-3333-7000-0000-000000000000",
    );
    expect(edge).toBeDefined();
    expect((edge as any)?.data?.bindings).toEqual([
      { output: "clipped_raster", input: "" },
    ]);
  });

  it("export round-trip preserves inputs (bare and explicit)", () => {
    const flow = useFlow();
    flow.yamlToNodes(PIPELINE_YAML);
    const exported = flow.exportPipelineObject();
    const yaml = YAML.stringify(exported);
    const parsed = YAML.parse(yaml);
    const clip = parsed.blocks.find(
      (b: any) => b.id === "019cf4bc-3333-7000-0000-000000000000",
    );
    expect(clip.inputs).toEqual([
      { block: "019cf4bc-2222-7000-0000-000000000000", output: "clipped_raster" },
    ]);
    const reproject = parsed.blocks.find(
      (b: any) => b.id === "019cf4bc-2222-7000-0000-000000000000",
    );
    expect(reproject.inputs).toEqual([
      "019cf4bc-1111-7000-0000-000000000000",
    ]);
  });

  it("yamlToNodes rejects YAML without a blocks array", () => {
    const flow = useFlow();
    expect(() => flow.yamlToNodes("name: foo\n")).toThrow();
  });
});
