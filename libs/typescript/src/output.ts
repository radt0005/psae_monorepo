import {
  readFileSync,
  existsSync,
  mkdirSync,
  copyFileSync,
  readdirSync,
  statSync,
} from "node:fs";
import { join, basename } from "node:path";
import yaml from "js-yaml";
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

const TYPE_TO_DEFAULT_NAME = new Map<Function, string>([
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

export function readBlockManifest(): Record<string, unknown> | null {
  const envPath = process.env["SPADE_BLOCK_MANIFEST"];
  if (envPath && existsSync(envPath)) {
    const content = readFileSync(envPath, "utf-8");
    const manifest = yaml.load(content) as Record<string, unknown> | null;
    return (manifest?.outputs as Record<string, unknown>) ?? null;
  }

  const blockYaml = "block.yaml";
  if (existsSync(blockYaml)) {
    const content = readFileSync(blockYaml, "utf-8");
    const manifest = yaml.load(content) as Record<string, unknown> | null;
    return (manifest?.outputs as Record<string, unknown>) ?? null;
  }

  return null;
}

export function inferOutputName(value: unknown): string {
  if (value !== null && typeof value === "object") {
    const name = TYPE_TO_DEFAULT_NAME.get(value.constructor);
    if (name) return name;
    return value.constructor.name.toLowerCase();
  }
  return "output";
}

function copyDirRecursive(src: string, dest: string): void {
  mkdirSync(dest, { recursive: true });
  for (const entry of readdirSync(src)) {
    const srcPath = join(src, entry);
    const destPath = join(dest, entry);
    if (statSync(srcPath).isDirectory()) {
      copyDirRecursive(srcPath, destPath);
    } else {
      copyFileSync(srcPath, destPath);
    }
  }
}

export function writeSingleOutput(name: string, value: unknown): void {
  const outputDir = join("outputs", name);
  mkdirSync(outputDir, { recursive: true });

  if (value instanceof FileCollection) {
    for (const filePath of value.paths) {
      copyFileSync(filePath, join(outputDir, basename(filePath)));
    }
  } else if (value instanceof Directory) {
    const src = value.path;
    for (const entry of readdirSync(src)) {
      const srcPath = join(src, entry);
      const destPath = join(outputDir, entry);
      if (statSync(srcPath).isDirectory()) {
        copyDirRecursive(srcPath, destPath);
      } else {
        copyFileSync(srcPath, destPath);
      }
    }
  } else if (value instanceof File) {
    copyFileSync(value.path, join(outputDir, basename(value.path)));
  }
}

export function writeOutputs(
  result: unknown,
  manifestOutputs: Record<string, unknown> | null = null
): void {
  if (result === null || result === undefined) {
    return;
  }

  mkdirSync("outputs", { recursive: true });

  if (result instanceof File || result instanceof Directory || result instanceof FileCollection) {
    let name: string;
    if (manifestOutputs && Object.keys(manifestOutputs).length === 1) {
      name = Object.keys(manifestOutputs)[0];
    } else {
      name = inferOutputName(result);
    }
    writeSingleOutput(name, result);
  } else if (typeof result === "object" && result !== null && !Array.isArray(result)) {
    for (const [key, value] of Object.entries(result)) {
      writeSingleOutput(key, value);
    }
  }
}
