import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import yaml from "js-yaml";

export function setupWorkDir(): { path: string; originalCwd: string } {
  const workDir = mkdtempSync(join(tmpdir(), "spade_test_"));
  mkdirSync(join(workDir, "inputs"));
  mkdirSync(join(workDir, "outputs"));
  mkdirSync(join(workDir, "logs"));
  const originalCwd = process.cwd();
  process.chdir(workDir);
  return { path: workDir, originalCwd };
}

export function createInputFile(
  name: string,
  filename: string = "data.tif",
  content: Buffer | string = "test data"
): string {
  const inputDir = join("inputs", name);
  mkdirSync(inputDir, { recursive: true });
  const filePath = join(inputDir, filename);
  writeFileSync(filePath, content);
  return filePath;
}

export function createInputCollection(
  name: string,
  filenames: string[],
  content: Buffer | string = "test data"
): string[] {
  const inputDir = join("inputs", name);
  mkdirSync(inputDir, { recursive: true });
  return filenames.map((filename) => {
    const filePath = join(inputDir, filename);
    writeFileSync(filePath, content);
    return filePath;
  });
}

export function writeParams(params: Record<string, unknown>): void {
  writeFileSync("params.yaml", yaml.dump(params));
}
