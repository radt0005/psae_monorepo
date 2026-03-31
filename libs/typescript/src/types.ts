export class File {
  readonly path: string;
  constructor(path: string) {
    this.path = path;
  }
}

export class Directory {
  readonly path: string;
  constructor(path: string) {
    this.path = path;
  }
}

export class RasterFile extends File {}
export class VectorFile extends File {}
export class TabularFile extends File {}
export class JsonFile extends File {}

export class FileCollection {
  readonly paths: string[];
  constructor(paths: string[]) {
    this.paths = paths;
  }
}

export class RasterFileCollection extends FileCollection {}
export class VectorFileCollection extends FileCollection {}
export class TabularFileCollection extends FileCollection {}
