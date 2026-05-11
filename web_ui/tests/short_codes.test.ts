import { describe, it, expect } from "vitest";
import { resolveShortCodes, isShortCode } from "../utils/short_codes";
import type { Pipeline } from "../utils/types";

const UUID_V7_REGEX =
  /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

describe("isShortCode", () => {
  const valid = ["@a", "@reproject", "@map_1", "@_x", "@A_B_2"];
  const invalid = [
    "@1bad",
    "@-x",
    "@",
    "foo",
    "@foo bar",
    "@foo.bar",
    "019cf4bc-1111-7000-0000-000000000000",
    "",
  ];

  for (const v of valid) {
    it(`accepts ${JSON.stringify(v)}`, () => {
      expect(isShortCode(v)).toBe(true);
    });
  }
  for (const v of invalid) {
    it(`rejects ${JSON.stringify(v)}`, () => {
      expect(isShortCode(v)).toBe(false);
    });
  }
});

describe("resolveShortCodes", () => {
  it("assigns fresh UUIDv7s to each unique short code", () => {
    const input: Pipeline = {
      id: "",
      name: "p",
      version: "1.0",
      blocks: [
        { id: "@source", name: "data.x", inputs: [], args: {} },
        { id: "@reproject", name: "data.y", inputs: ["@source"], args: {} },
        {
          id: "@clip",
          name: "data.z",
          inputs: [{ block: "@reproject", output: "tiles" }],
          args: {},
        },
      ],
    };
    const out = resolveShortCodes(input);

    expect(out.blocks).toHaveLength(3);
    for (const b of out.blocks) {
      expect(b.id).toMatch(UUID_V7_REGEX);
    }
    // Same short code repeated → same UUID.
    expect(out.blocks[1].inputs[0]).toBe(out.blocks[0].id);
    const explicit = out.blocks[2].inputs[0];
    expect(typeof explicit).not.toBe("string");
    if (typeof explicit !== "string") {
      expect(explicit.block).toBe(out.blocks[1].id);
      expect(explicit.output).toBe("tiles");
    }
  });

  it("does not mutate the input pipeline", () => {
    const input: Pipeline = {
      id: "",
      name: "p",
      version: "1.0",
      blocks: [{ id: "@a", name: "x", inputs: [], args: {} }],
    };
    const snapshot = JSON.stringify(input);
    resolveShortCodes(input);
    expect(JSON.stringify(input)).toBe(snapshot);
  });

  it("preserves args values verbatim", () => {
    const input: Pipeline = {
      id: "",
      name: "p",
      version: "1.0",
      blocks: [
        {
          id: "@a",
          name: "x",
          inputs: [],
          args: { tag: "@nope", nested: { deeper: "@still_nope" } },
        },
      ],
    };
    const out = resolveShortCodes(input);
    expect(out.blocks[0].args.tag).toBe("@nope");
    expect((out.blocks[0].args.nested as Record<string, unknown>).deeper).toBe(
      "@still_nope",
    );
  });

  it("preserves mixed UUID + short-code pipelines", () => {
    const concrete = "019cf4bc-aaaa-7000-0000-000000000001";
    const input: Pipeline = {
      id: "",
      name: "p",
      version: "1.0",
      blocks: [
        { id: concrete, name: "a", inputs: [], args: {} },
        {
          id: "@b",
          name: "b",
          inputs: [concrete, { block: "@b", output: "x" }],
          args: {},
        },
      ],
    };
    const out = resolveShortCodes(input);
    expect(out.blocks[0].id).toBe(concrete);
    expect(out.blocks[1].id).toMatch(UUID_V7_REGEX);
    expect(out.blocks[1].inputs[0]).toBe(concrete);
    const explicit = out.blocks[1].inputs[1];
    if (typeof explicit !== "string") {
      expect(explicit.block).toBe(out.blocks[1].id);
    }
  });

  it("rejects a top-level short code id", () => {
    const input: Pipeline = {
      id: "@whatever",
      name: "p",
      version: "1.0",
      blocks: [],
    };
    expect(() => resolveShortCodes(input)).toThrow(/pipeline-level/);
  });

  it("emits a Pipeline whose ids parse as UUIDv7s", () => {
    const input: Pipeline = {
      id: "",
      name: "p",
      version: "1.0",
      blocks: [
        { id: "@a", name: "x", inputs: [], args: {} },
        { id: "@b", name: "y", inputs: ["@a"], args: {} },
      ],
    };
    const out = resolveShortCodes(input);
    for (const b of out.blocks) {
      expect(b.id).toMatch(UUID_V7_REGEX);
    }
    expect(out.blocks[1].inputs[0]).toMatch(UUID_V7_REGEX);
  });
});
