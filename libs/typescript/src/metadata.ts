import type {
  File,
  Directory,
  FileCollection,
} from "./types.ts";

export type SpadeTypeClass =
  | typeof File
  | typeof Directory
  | typeof FileCollection
  | (new (path: string) => File)
  | (new (path: string) => Directory)
  | (new (paths: string[]) => FileCollection)
  | "string"
  | "number"
  | "boolean";

export interface SpadeMetadata {
  inputs: Record<string, SpadeTypeClass>;
  output?: SpadeTypeClass;
  description?: string;
}

const metadataStore = new WeakMap<Function, SpadeMetadata>();

export function setMetadata(fn: Function, metadata: SpadeMetadata): void {
  metadataStore.set(fn, metadata);
}

export function getMetadata(fn: Function): SpadeMetadata | undefined {
  return metadataStore.get(fn);
}

export function spadeBlock<T extends Function>(metadata: SpadeMetadata) {
  return (fn: T): T => {
    setMetadata(fn, metadata);
    return fn;
  };
}
