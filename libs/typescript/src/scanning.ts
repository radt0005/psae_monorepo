import { readFileSync, existsSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
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
import type { SpadeMetadata, SpadeTypeClass } from "./metadata.ts";

export function loadParams(): Record<string, unknown> {
  const paramsPath = "params.yaml";
  if (!existsSync(paramsPath)) {
    return {};
  }
  const content = readFileSync(paramsPath, "utf-8");
  const params = yaml.load(content);
  return (params as Record<string, unknown>) ?? {};
}

function isFileSubclass(
  cls: SpadeTypeClass
): cls is new (path: string) => File {
  return typeof cls === "function" && cls.prototype instanceof File || cls === File;
}

function isDirectorySubclass(
  cls: SpadeTypeClass
): cls is new (path: string) => Directory {
  return typeof cls === "function" && (cls.prototype instanceof Directory || cls === Directory);
}

function isFileCollectionSubclass(
  cls: SpadeTypeClass
): cls is new (paths: string[]) => FileCollection {
  return typeof cls === "function" && (cls.prototype instanceof FileCollection || cls === FileCollection);
}

export function scanInputs(
  typeHints: Record<string, SpadeTypeClass>
): Record<string, unknown> {
  const inputsDir = "inputs";
  if (!existsSync(inputsDir)) {
    return {};
  }

  const result: Record<string, unknown> = {};
  const subdirs = readdirSync(inputsDir).sort();

  for (const entry of subdirs) {
    const subdirPath = join(inputsDir, entry);
    if (!statSync(subdirPath).isDirectory()) {
      continue;
    }

    const name = entry;
    const expectedType = typeHints[name];

    const files = readdirSync(subdirPath)
      .sort()
      .filter((f) => statSync(join(subdirPath, f)).isFile())
      .map((f) => join(subdirPath, f));

    if (expectedType !== undefined) {
      if (isDirectorySubclass(expectedType)) {
        result[name] = new expectedType(subdirPath);
        continue;
      }
      if (isFileCollectionSubclass(expectedType)) {
        result[name] = new expectedType(files);
        continue;
      }
      if (isFileSubclass(expectedType)) {
        if (files.length === 0) {
          throw new Error(
            `Input '${name}' expects a file but directory '${subdirPath}' is empty`
          );
        }
        result[name] = new expectedType(files[0]);
        continue;
      }
    }

    // Default inference
    if (files.length === 0) {
      throw new Error(`Input directory '${subdirPath}' is empty`);
    }
    if (files.length === 1) {
      result[name] = new File(files[0]);
    } else {
      result[name] = new FileCollection(files);
    }
  }

  return result;
}

export function buildFunctionArgs(
  metadata: SpadeMetadata | undefined
): Record<string, unknown> {
  const typeHints: Record<string, SpadeTypeClass> = metadata?.inputs ?? {};
  const params = loadParams();
  const inputs = scanInputs(typeHints);
  return { ...params, ...inputs };
}
