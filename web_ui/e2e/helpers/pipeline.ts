import { expect, type Page } from "@playwright/test";

/**
 * A minimal spec-conformant pipeline using short codes (spec/pipeline.md §6.1).
 * One block, no inputs — passes `resolveShortCodes` + `validatePipeline` (the
 * server validates structure without manifests, so the block `name` need not
 * be installed for the run to be accepted).
 */
export function minimalPipelineYaml(name = "e2e-pipeline"): string {
  return [
    `name: ${name}`,
    `version: 0.1.0`,
    `blocks:`,
    `  - id: "@source"`,
    `    name: noop`,
    `    inputs: []`,
    `    args: {}`,
    ``,
  ].join("\n");
}

export interface PipelineRow {
  id: string;
  name: string;
  visibility: "private" | "shared" | "public";
  ownerId: string;
}

/** Create a saved pipeline through the API on the page's session. */
export async function createPipelineViaApi(
  page: Page,
  opts: {
    name: string;
    yaml?: string;
    visibility?: "private" | "shared" | "public";
  },
): Promise<PipelineRow> {
  const res = await page.request.post("/api/pipelines", {
    data: {
      name: opts.name,
      yaml: opts.yaml ?? minimalPipelineYaml(opts.name),
      visibility: opts.visibility ?? "private",
    },
  });
  expect(res.ok(), `create pipeline: ${res.status()} ${await res.text()}`).toBeTruthy();
  return res.json();
}

/** Share a pipeline with another user by email. */
export async function sharePipelineViaApi(
  page: Page,
  pipelineId: string,
  email: string,
) {
  const res = await page.request.post(`/api/pipelines/${pipelineId}/share`, {
    data: { email },
  });
  expect(res.ok(), `share pipeline: ${res.status()} ${await res.text()}`).toBeTruthy();
  return res.json();
}

export interface RunRow {
  id: string;
  name: string | null;
  status: string;
}

/** Submit a run through the API on the page's session. */
export async function submitRunViaApi(
  page: Page,
  opts: { yaml?: string; name?: string; pipelineId?: string },
): Promise<RunRow> {
  const data: Record<string, unknown> = {};
  if (opts.pipelineId) data.pipelineId = opts.pipelineId;
  else data.yaml = opts.yaml ?? minimalPipelineYaml(opts.name ?? "e2e-run");
  if (opts.name) data.name = opts.name;

  const res = await page.request.post("/api/runs", { data });
  expect(res.ok(), `submit run: ${res.status()} ${await res.text()}`).toBeTruthy();
  return res.json();
}
