import { describe, it, expect } from "vitest";
import {
  computeMapContext,
  isBroadcastEdge,
  activeMapFor,
  type MapGraphNode,
} from "~/utils/mapContext";

const n = (
  id: string,
  kind: MapGraphNode["kind"],
  deps: string[] = [],
): MapGraphNode => ({ id, kind, deps });

describe("computeMapContext", () => {
  it("marks blocks between a map and a reduce as in-context", () => {
    // src → map → work → reduce → sink
    const r = computeMapContext([
      n("src", "standard"),
      n("map", "map", ["src"]),
      n("work", "standard", ["map"]),
      n("reduce", "reduce", ["work"]),
      n("sink", "standard", ["reduce"]),
    ]);

    // The map block itself is the opener — not yet in a context.
    expect(r.inContext.has("map")).toBe(false);
    // Downstream of the map, up to and including the reduce, is in-context.
    expect(r.inContext.has("work")).toBe(true);
    expect(r.inContext.has("reduce")).toBe(true);
    // The reduce closes the context, so its downstream is back out.
    expect(r.inContext.has("sink")).toBe(false);
    expect(r.inContext.has("src")).toBe(false);
  });

  it("tracks the active map id for in-context blocks", () => {
    const r = computeMapContext([
      n("map", "map"),
      n("work", "standard", ["map"]),
      n("reduce", "reduce", ["work"]),
    ]);
    expect(activeMapFor(r, "work")).toBe("map");
    expect(activeMapFor(r, "reduce")).toBe("map");
    expect(activeMapFor(r, "map")).toBeNull();
  });

  it("flags a nested map opened inside an existing context", () => {
    // map1 → map2 (nested!) → reduce2 → reduce1
    const r = computeMapContext([
      n("map1", "map"),
      n("map2", "map", ["map1"]),
      n("reduce2", "reduce", ["map2"]),
      n("reduce1", "reduce", ["reduce2"]),
    ]);
    expect(r.nestedMaps).toContain("map2");
    expect(r.nestedMaps).not.toContain("map1");
  });

  it("does not flag sequential (non-nested) map/reduce pairs", () => {
    // map1 → reduce1 → map2 → reduce2
    const r = computeMapContext([
      n("map1", "map"),
      n("reduce1", "reduce", ["map1"]),
      n("map2", "map", ["reduce1"]),
      n("reduce2", "reduce", ["map2"]),
    ]);
    expect(r.nestedMaps).toHaveLength(0);
  });

  it("detects a broadcast input from a non-mapped dependency", () => {
    // map → work ← config(standard, outside the context)
    // work is in map's context; config feeds it from outside → broadcast.
    const r = computeMapContext([
      n("config", "standard"),
      n("map", "map"),
      n("work", "standard", ["map", "config"]),
      n("reduce", "reduce", ["work"]),
    ]);
    expect(isBroadcastEdge(r, "config", "work")).toBe(true);
    // The mapped edge (map → work) is not a broadcast.
    expect(isBroadcastEdge(r, "map", "work")).toBe(false);
  });

  it("an edge into a non-mapped block is never a broadcast", () => {
    const r = computeMapContext([
      n("a", "standard"),
      n("b", "standard", ["a"]),
    ]);
    expect(isBroadcastEdge(r, "a", "b")).toBe(false);
  });

  it("propagates context across a broadcast diamond without shrinking it", () => {
    // map → x → work ; map → work (direct). config feeds work from outside.
    const r = computeMapContext([
      n("config", "standard"),
      n("map", "map"),
      n("x", "standard", ["map"]),
      n("work", "standard", ["x", "config"]),
      n("reduce", "reduce", ["work"]),
    ]);
    expect(r.inContext.has("x")).toBe(true);
    expect(r.inContext.has("work")).toBe(true);
    expect(isBroadcastEdge(r, "config", "work")).toBe(true);
  });

  it("handles an empty graph and graphs with no map blocks", () => {
    expect(computeMapContext([]).inContext.size).toBe(0);
    const r = computeMapContext([
      n("a", "standard"),
      n("b", "standard", ["a"]),
      n("c", "standard", ["b"]),
    ]);
    expect(r.inContext.size).toBe(0);
    expect(r.nestedMaps).toHaveLength(0);
  });

  it("ignores dangling deps that aren't in the graph", () => {
    const r = computeMapContext([
      n("map", "map", ["ghost"]),
      n("work", "standard", ["map"]),
    ]);
    expect(r.inContext.has("work")).toBe(true);
  });
});
