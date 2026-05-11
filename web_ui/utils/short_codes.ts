import { v7 as uuidv7 } from "uuid";
import type { Pipeline, Block, InputRef } from "./types";

/**
 * Short-code grammar from spec/pipeline.md §6.1: `@<identifier>` where
 * `<identifier>` is `[A-Za-z_][A-Za-z0-9_]*`.
 */
const SHORT_CODE_REGEX = /^@[A-Za-z_][A-Za-z0-9_]*$/;

export function isShortCode(s: unknown): s is string {
  return typeof s === "string" && SHORT_CODE_REGEX.test(s);
}

/**
 * Resolve short codes in a Pipeline by minting fresh UUIDv7s.  Per
 * spec/pipeline.md §6.7, the web UI does NOT persist a lockfile -- each
 * upload mints fresh UUIDs.  Within a single call, a repeated short code
 * (e.g. `@source` used as a block id and again as an input reference)
 * resolves to the *same* UUID.
 *
 * The walk is structure-aware: only `block.id` and `block.inputs[i]` are
 * substituted.  `args` values and the pipeline-level `id` are not
 * touched.  A top-level `id` that holds a short code throws.
 *
 * Returns a new Pipeline; the input object is not mutated.
 */
export function resolveShortCodes(pipeline: Pipeline): Pipeline {
  if (isShortCode(pipeline.id)) {
    throw new Error(
      `short code ${pipeline.id!} is not allowed on the pipeline-level 'id' field ` +
        `(spec/pipeline.md §6.1): omit 'id' to let the system generate one, ` +
        `or use a concrete UUID`,
    );
  }

  const bindings = new Map<string, string>();
  const resolve = (code: string): string => {
    const existing = bindings.get(code);
    if (existing !== undefined) return existing;
    const fresh = uuidv7();
    bindings.set(code, fresh);
    return fresh;
  };

  const newBlocks: Block[] = pipeline.blocks.map((block) => {
    const newId = isShortCode(block.id) ? resolve(block.id) : block.id;
    const newInputs: InputRef[] = block.inputs.map((ref) => {
      if (typeof ref === "string") {
        return isShortCode(ref) ? resolve(ref) : ref;
      }
      // Explicit form: substitute only `block`, never `output`.
      return {
        block: isShortCode(ref.block) ? resolve(ref.block) : ref.block,
        output: ref.output,
      };
    });
    // `args` is application data and must pass through verbatim.
    return {
      id: newId,
      name: block.name,
      inputs: newInputs,
      args: block.args,
    };
  });

  return {
    id: pipeline.id,
    name: pipeline.name,
    version: pipeline.version,
    description: pipeline.description,
    blocks: newBlocks,
  };
}
