import {
  File,
  Directory,
  FileCollection,
  RasterFile,
  VectorFile,
  TabularFile,
  JsonFile,
  RasterFileCollection,
  VectorFileCollection,
  TabularFileCollection,
} from "./types.ts";
import { getMetadata, type SpadeTypeClass } from "./metadata.ts";

const TS_TYPE_TO_MANIFEST = new Map<SpadeTypeClass, Record<string, string>>([
  [File, { type: "file" }],
  [RasterFile, { type: "file", format: "GeoTIFF" }],
  [VectorFile, { type: "file", format: "GeoJSON" }],
  [TabularFile, { type: "file", format: "CSV" }],
  [JsonFile, { type: "json" }],
  [Directory, { type: "directory" }],
  [FileCollection, { type: "collection", item_type: "file" }],
  [RasterFileCollection, { type: "collection", item_type: "file", format: "GeoTIFF" }],
  [VectorFileCollection, { type: "collection", item_type: "file", format: "GeoJSON" }],
  [TabularFileCollection, { type: "collection", item_type: "file", format: "CSV" }],
  ["string", { type: "string" }],
  ["number", { type: "number" }],
  ["boolean", { type: "boolean" }],
]);

const TYPE_TO_OUTPUT_NAME = new Map<SpadeTypeClass, string>([
  [File, "file"],
  [RasterFile, "raster"],
  [VectorFile, "vector"],
  [TabularFile, "tabular"],
  [JsonFile, "json"],
  [Directory, "directory"],
  [FileCollection, "files"],
  [RasterFileCollection, "rasters"],
  [VectorFileCollection, "vectors"],
  [TabularFileCollection, "tables"],
]);

export function build(fn: Function): Record<string, unknown> {
  const metadata = getMetadata(fn);
  if (!metadata) {
    return { inputs: {}, outputs: {} };
  }

  const inputs: Record<string, Record<string, string>> = {};
  for (const [paramName, paramType] of Object.entries(metadata.inputs)) {
    const entry = TS_TYPE_TO_MANIFEST.get(paramType);
    if (entry) {
      inputs[paramName] = { ...entry };
    }
  }

  const outputs: Record<string, Record<string, string>> = {};
  if (metadata.output !== undefined) {
    const entry = TS_TYPE_TO_MANIFEST.get(metadata.output);
    if (entry) {
      const outputName = TYPE_TO_OUTPUT_NAME.get(metadata.output) ?? "output";
      outputs[outputName] = { ...entry };
    }
  }

  const manifest: Record<string, unknown> = {};
  if (metadata.description) {
    manifest.description = metadata.description;
  }
  manifest.inputs = inputs;
  manifest.outputs = outputs;

  return manifest;
}
