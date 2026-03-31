# TypeScript Spade Library — Implementation Plan

This plan implements the Spade runtime library for TypeScript (Bun), following the specifications in `../../spec/` and matching the patterns established by the Python and R reference implementations.

## Phase 1: Project Setup & Dependencies

- [DONE] Add runtime dependencies to `package.json`: `js-yaml` for YAML parsing (the only non-dev dependency needed).
- [DONE] Add dev dependencies: `@types/js-yaml`, `@types/bun` (already present).
- [DONE] Configure `package.json` with `"exports"` pointing to `src/index.ts` and ensure `"type": "module"` is set.
- [DONE] Run `bun install` to install all dependencies.

## Phase 2: Type System (`src/types.ts`)

Define the core Spade type classes. Each type is a simple class with either a `path: string` (single file/directory) or `paths: string[]` (collections) property. TypeScript classes are used instead of Pydantic models, providing `instanceof` checks for type-based dispatch.

- [DONE] Define `File` class with `path: string` constructor parameter. This is the base class for all single-file types.
- [DONE] Define `Directory` class with `path: string` constructor parameter.
- [DONE] Define `RasterFile` class extending `File` (no additional fields).
- [DONE] Define `VectorFile` class extending `File` (no additional fields).
- [DONE] Define `TabularFile` class extending `File` (no additional fields).
- [DONE] Define `JsonFile` class extending `File` (no additional fields).
- [DONE] Define `FileCollection` class with `paths: string[]` constructor parameter. This is the base class for all collection types.
- [DONE] Define `RasterFileCollection` class extending `FileCollection` (no additional fields).
- [DONE] Define `VectorFileCollection` class extending `FileCollection` (no additional fields).
- [DONE] Define `TabularFileCollection` class extending `FileCollection` (no additional fields).
- [DONE] Add a static `typeName` readonly property to each class (e.g., `"File"`, `"RasterFile"`, `"RasterFileCollection"`) to enable the type-hint metadata system used by `run()` and `build()`. (Used instanceof + constructor identity instead — more idiomatic for TypeScript.)

## Phase 3: Type Hint Metadata System (`src/metadata.ts`)

TypeScript does not have Python-style runtime type hints on function parameters. We need a mechanism for block authors to declare input/output types. This is analogous to R's `spade_types()` annotation system.

- [DONE] Define a `SpadeTypeClass` type alias representing the union of all Spade type constructors (e.g., `typeof File | typeof RasterFile | ...`) plus scalar type string literals (`"string"`, `"number"`, `"boolean"`).
- [DONE] Define a `SpadeMetadata` interface: `{ inputs: Record<string, SpadeTypeClass>; output?: SpadeTypeClass; description?: string }`.
- [DONE] Implement a `WeakMap<Function, SpadeMetadata>`-backed `setMetadata(fn, metadata)` function that associates a handler function with its type metadata.
- [DONE] Implement a `getMetadata(fn): SpadeMetadata | undefined` function that retrieves the metadata.
- [DONE] Implement a convenience `spadeBlock(metadata: SpadeMetadata)` decorator/wrapper function that both stores metadata and returns the original function, enabling usage like:
  ```ts
  const handler = spadeBlock({
    inputs: { raster: RasterFile, buffer: "number" },
    output: RasterFile,
    description: "Reprojects a raster",
  })(function (raster: RasterFile, buffer: number) {
    // ...
  });
  ```
  This provides a clean, ergonomic API equivalent to Python's type hints and R's `spade_types()`.

## Phase 4: Parameter Loading (`src/scanning.ts` — `loadParams()`)

- [DONE] Implement `loadParams(): Record<string, unknown>` that reads `params.yaml` from the current working directory.
- [DONE] If `params.yaml` does not exist, return `{}`.
- [DONE] If `params.yaml` exists but is empty or parses to `null`, return `{}`.
- [DONE] Use `js-yaml`'s `load()` with safe schema for YAML parsing.

## Phase 5: Input Scanning (`src/scanning.ts` — `scanInputs()`)

- [DONE] Implement `scanInputs(typeHints: Record<string, SpadeTypeClass>): Record<string, unknown>` that scans the `inputs/` directory.
- [DONE] If `inputs/` directory does not exist, return `{}`.
- [DONE] Iterate over each subdirectory of `inputs/` in **sorted** order (alphabetical). Skip non-directory entries.
- [DONE] For each subdirectory, the subdirectory name becomes the parameter name.
- [DONE] List all files within the subdirectory in **sorted** order (Bun's `fs` or Node's `readdirSync`).
- [DONE] If a type hint exists for this parameter name:
  - If the hint is `Directory`: construct `new Directory(subdirPath)` (or the appropriate Directory subclass).
  - If the hint is a `FileCollection` subclass: construct with `paths` set to all file paths in the subdirectory.
  - If the hint is a `File` subclass: verify at least one file exists (throw `Error` if empty), construct with `path` set to the first file.
- [DONE] If no type hint exists, use default inference:
  - 0 files: throw `Error("Input directory '<name>' is empty")`.
  - 1 file: construct `new File(path)`.
  - 2+ files: construct `new FileCollection(paths)`.

## Phase 6: Argument Building (`src/scanning.ts` — `buildFunctionArgs()`)

- [DONE] Implement `buildFunctionArgs(metadata: SpadeMetadata | undefined): Record<string, unknown>`.
- [DONE] Extract type hints from `metadata.inputs` (or `{}` if no metadata).
- [DONE] Call `loadParams()` to get scalar parameters.
- [DONE] Call `scanInputs(typeHints)` to get file-based inputs.
- [DONE] Merge: `{ ...params, ...inputs }` — inputs take precedence over params when names collide.

## Phase 7: Block Manifest Reading (`src/output.ts` — `readBlockManifest()`)

- [DONE] Implement `readBlockManifest(): Record<string, unknown> | null`.
- [DONE] Check `SPADE_BLOCK_MANIFEST` environment variable first. If set and the file exists, parse it as YAML and return the `outputs` key (or `null` if missing).
- [DONE] If env var not set or file doesn't exist, check for `block.yaml` in the current working directory. Parse and return `outputs` key.
- [DONE] If neither source exists, return `null`.

## Phase 8: Output Writing (`src/output.ts` — `writeOutputs()`)

- [DONE] Define `TYPE_TO_DEFAULT_NAME` mapping from each type class to its default output directory name:
  - `File` → `"file"`, `RasterFile` → `"raster"`, `VectorFile` → `"vector"`, `TabularFile` → `"tabular"`, `JsonFile` → `"json"`, `Directory` → `"directory"`, `FileCollection` → `"files"`, `RasterFileCollection` → `"rasters"`, `VectorFileCollection` → `"vectors"`, `TabularFileCollection` → `"tables"`.
- [DONE] Implement `inferOutputName(value: unknown): string` that looks up the value's constructor in `TYPE_TO_DEFAULT_NAME`, falling back to the lowercase constructor name.
- [DONE] Implement `writeSingleOutput(name: string, value: unknown): void`:
  - Create `outputs/<name>/` directory (recursive mkdir).
  - If value is a `FileCollection` instance: copy each file in `paths` to the output directory, preserving filenames.
  - If value is a `Directory` instance: copy all contents of `value.path` into the output directory (files and subdirectories).
  - If value is a `File` instance: copy `value.path` to the output directory, preserving the filename.
  - Use `Bun.file()` + `Bun.write()` or Node's `fs.copyFileSync` for file copies. For directory copies, recursively copy entries.
- [DONE] Implement `writeOutputs(result: unknown, manifestOutputs: Record<string, unknown> | null): void`:
  - If `result` is `null` or `undefined`, return immediately (no output).
  - Ensure `outputs/` directory exists.
  - If `result` is a plain object (dict pattern): iterate entries, call `writeSingleOutput(key, value)` for each.
  - If `result` is a `File`, `Directory`, or `FileCollection` instance:
    - If `manifestOutputs` has exactly 1 key, use that key as the output name.
    - Otherwise, infer name from `inferOutputName(result)`.
    - Call `writeSingleOutput(name, result)`.

## Phase 9: The `run()` Function (`src/run.ts`)

- [DONE] Implement `run(fn: Function): void` (synchronous) or `run(fn: Function): Promise<void>` (async, to support async handlers).
- [DONE] Retrieve metadata via `getMetadata(fn)`.
- [DONE] Call `buildFunctionArgs(metadata)` to build the full arguments dictionary.
- [DONE] Determine the handler's expected parameter names from `metadata.inputs` keys. If metadata is not available, pass all args.
- [DONE] Filter `args` to only include keys present in the handler's declared inputs (unless no metadata, in which case pass everything).
- [DONE] Call the handler with spread arguments. Since TypeScript functions take positional args (not **kwargs), the handler should accept a single object parameter or we should call it with the args object. **Design decision**: The handler signature should accept a single destructured object, e.g.:
  ```ts
  function handler({ raster, buffer }: { raster: RasterFile; buffer: number }): RasterFile { ... }
  ```
  Alternatively, we call `fn(args)` passing the full args dict as the single argument. This is the cleanest TypeScript-idiomatic approach and matches how TypeScript functions naturally work with named parameters.
- [DONE] If the handler returns a `Promise`, await it.
- [DONE] Read the block manifest via `readBlockManifest()`.
- [DONE] Write outputs via `writeOutputs(result, manifestOutputs)`.

## Phase 10: The `build()` Function (`src/build.ts`)

Generates a block manifest dictionary from the handler's metadata. This is used by `spade add` and `spade check`.

- [DONE] Define `TS_TYPE_TO_MANIFEST` mapping:
  - `File` → `{ type: "file" }`
  - `RasterFile` → `{ type: "file", format: "GeoTIFF" }`
  - `VectorFile` → `{ type: "file", format: "GeoJSON" }`
  - `TabularFile` → `{ type: "file", format: "CSV" }`
  - `JsonFile` → `{ type: "json" }`
  - `Directory` → `{ type: "directory" }`
  - `FileCollection` → `{ type: "collection", item_type: "file" }`
  - `RasterFileCollection` → `{ type: "collection", item_type: "file", format: "GeoTIFF" }`
  - `VectorFileCollection` → `{ type: "collection", item_type: "file", format: "GeoJSON" }`
  - `TabularFileCollection` → `{ type: "collection", item_type: "file", format: "CSV" }`
  - `"string"` → `{ type: "string" }`
  - `"number"` → `{ type: "number" }`
  - `"boolean"` → `{ type: "boolean" }`
- [DONE] Define `TYPE_TO_OUTPUT_NAME` mapping (same as `TYPE_TO_DEFAULT_NAME` from Phase 8, but only for class types, not scalar strings).
- [DONE] Implement `build(fn: Function): Record<string, unknown>`:
  - Retrieve metadata via `getMetadata(fn)`.
  - If no metadata, return `{ inputs: {}, outputs: {} }`.
  - For each entry in `metadata.inputs`, look up the manifest entry in `TS_TYPE_TO_MANIFEST` and add to `inputs`.
  - If `metadata.output` is defined and not `undefined`/`null`, look up the manifest entry and add to `outputs` using the appropriate output name from `TYPE_TO_OUTPUT_NAME`.
  - If `metadata.description` is set, include it in the manifest.
  - Return the manifest object `{ description?, inputs, outputs }`.

## Phase 11: Public API & Exports (`src/index.ts`)

- [DONE] Re-export all types from `src/types.ts`: `File`, `Directory`, `RasterFile`, `VectorFile`, `TabularFile`, `JsonFile`, `FileCollection`, `RasterFileCollection`, `VectorFileCollection`, `TabularFileCollection`.
- [DONE] Re-export `run` from `src/run.ts`.
- [DONE] Re-export `build` from `src/build.ts`.
- [DONE] Re-export `spadeBlock` and `setMetadata` from `src/metadata.ts`.
- [DONE] Update root `index.ts` to re-export from `src/index.ts`.

## Phase 12: Tests — Type System (`tests/types.test.ts`)

- [DONE] Test `File` construction: verify `path` property is set correctly.
- [DONE] Test `RasterFile` extends `File`: `instanceof File` returns `true`.
- [DONE] Test `VectorFile` extends `File`: `instanceof File` returns `true`.
- [DONE] Test `TabularFile` extends `File`: `instanceof File` returns `true`.
- [DONE] Test `JsonFile` extends `File`: `instanceof File` returns `true`.
- [DONE] Test `Directory` construction: verify `path` property is set correctly.
- [DONE] Test `FileCollection` construction: verify `paths` property is set correctly.
- [DONE] Test `RasterFileCollection` extends `FileCollection`: `instanceof FileCollection` returns `true`.
- [DONE] Test `VectorFileCollection` extends `FileCollection`: `instanceof FileCollection` returns `true`.
- [DONE] Test `TabularFileCollection` extends `FileCollection`: `instanceof FileCollection` returns `true`.

## Phase 13: Tests — Parameter Loading (`tests/scanning.test.ts`)

Implement test helpers first:
- [DONE] Create a `setupWorkDir()` helper that creates a temporary directory with `inputs/`, `outputs/`, `logs/` subdirectories, changes `process.cwd()` to that directory, and returns a cleanup function.
- [DONE] Create a `createInputFile(name, filename, content)` helper that creates `inputs/<name>/<filename>` with the given content.
- [DONE] Create a `createInputCollection(name, filenames, content)` helper that creates multiple files in `inputs/<name>/`.
- [DONE] Create a `writeParams(params)` helper that writes `params.yaml` with the given object.

Tests for `loadParams()`:
- [DONE] Test basic params loading: write `{ buffer_distance: 30, method: "bilinear" }` to `params.yaml`, verify `loadParams()` returns the correct object.
- [DONE] Test empty params file: write empty string to `params.yaml`, verify returns `{}`.
- [DONE] Test missing params file: verify returns `{}` when no `params.yaml` exists.

Tests for `scanInputs()`:
- [DONE] Test single file input with type hint: create `inputs/raster/data.tif`, call `scanInputs({ raster: RasterFile })`, verify result is a `RasterFile` instance with correct path.
- [DONE] Test untyped single file defaults to `File`: create `inputs/source/data.tif`, call `scanInputs({})`, verify result is `File`.
- [DONE] Test `Directory` input: create `inputs/source/` with files, call with `{ source: Directory }`, verify result is `Directory`.
- [DONE] Test collection input: create `inputs/tiles/` with 3 files, call with `{ tiles: RasterFileCollection }`, verify paths length is 3.
- [DONE] Test multiple inputs: create `inputs/reference/` and `inputs/target/`, verify both present in result.
- [DONE] Test empty input directory with type hint throws error.
- [DONE] Test untyped multiple files defaults to `FileCollection`.
- [DONE] Test no `inputs/` directory returns `{}`.

Tests for `buildFunctionArgs()`:
- [DONE] Test params and inputs are merged correctly.
- [DONE] Test inputs take precedence over params on name collision.

## Phase 14: Tests — Output Writing (`tests/output.test.ts`)

- [DONE] Test `inferOutputName()`: verify correct default name for each type (`File` → `"file"`, `RasterFile` → `"raster"`, etc.).
- [DONE] Test `writeOutputs(null)`: verify no output files created.
- [DONE] Test single `RasterFile` output: verify file copied to `outputs/raster/<filename>`.
- [DONE] Test single file output with manifest: verify manifest output name is used instead of inferred name.
- [DONE] Test dict output: verify each key creates its own subdirectory with the correct file.
- [DONE] Test `FileCollection` output: verify all files copied to `outputs/<name>/`.
- [DONE] Test `Directory` output: verify directory contents copied to `outputs/directory/`.
- [DONE] Test filename preservation: verify original filename is kept in output.

Tests for `readBlockManifest()`:
- [DONE] Test no manifest returns `null`.
- [DONE] Test `block.yaml` in CWD: verify outputs section is returned.
- [DONE] Test `SPADE_BLOCK_MANIFEST` env var: verify the env var path takes priority.

## Phase 15: Tests — `build()` Function (`tests/build.test.ts`)

- [DONE] Test simple function with `File` input: verify manifest has correct input entry.
- [DONE] Test typed inputs (`RasterFile`, `VectorFile`): verify format fields in manifest.
- [DONE] Test scalar inputs (`"string"`, `"number"`, `"boolean"`): verify manifest type fields.
- [DONE] Test with return type: verify outputs section populated correctly.
- [DONE] Test with description: verify description field in manifest.
- [DONE] Test no metadata: verify empty inputs and outputs.
- [DONE] Test collection input: verify `type: "collection"` with `item_type`.
- [DONE] Test no return type (output undefined): verify empty outputs.
- [DONE] Test mixed inputs (file types + scalars + return type + description): full integration check.

## Phase 16: Tests — `run()` Function Integration (`tests/run.test.ts`)

- [DONE] Test simple handler: create input file, define handler that receives `File`, verify handler is called with correct argument.
- [DONE] Test with params and inputs: write `params.yaml` + create input files, verify handler receives all args.
- [DONE] Test with typed inputs: verify `RasterFile` type is correctly instantiated.
- [DONE] Test with output: handler returns `RasterFile`, verify file appears in `outputs/`.
- [DONE] Test with dict output: handler returns object with multiple outputs, verify all appear.
- [DONE] Test handler exception propagates: handler throws, verify error propagates out of `run()`.
- [DONE] Test no return value: handler returns nothing, verify no output files.
- [DONE] Test extra params are filtered: params has keys not in metadata inputs, verify handler only receives declared inputs.
- [DONE] Test full end-to-end workflow: params + multiple typed inputs → handler processes → typed output written.

## Phase 17: Verify & Polish

- [DONE] Run `bun test` and confirm all tests pass.
- [DONE] Verify the public API is complete: all types, `run`, `build`, `spadeBlock` are exported.
- [DONE] Verify consistency with Python implementation: same behavior for same inputs/outputs.
- [DONE] Verify `bun build` can bundle the library (as required by `spade install` for TypeScript).
