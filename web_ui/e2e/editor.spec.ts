import { test, expect } from "@playwright/test";
import YAML from "yaml";
import { makeUser } from "./helpers/users";
import { registerViaApi } from "./helpers/auth";

/**
 * Phase G: YAML round-trip (scenario "round-trip test") and the ambiguous
 * connection-resolution flow (scenario 2).
 */

// Two fixed UUIDv7 ids so import → export comparison is exact (no minting).
const A = "0192d3c0-0000-7000-8000-000000000001";
const B = "0192d3c0-0000-7000-8000-000000000002";

const sourceYaml = [
  "name: roundtrip-pipeline",
  "version: 0.1.0",
  "blocks:",
  `  - id: ${A}`,
  "    name: loader",
  "    inputs: []",
  "    args: {}",
  `  - id: ${B}`,
  "    name: transform",
  "    inputs:",
  `      - ${A}`,
  "    args: {}",
  "",
].join("\n");

test.describe("editor", () => {
  test("import a pipeline, export it, structure round-trips", async ({
    page,
  }) => {
    await registerViaApi(page, makeUser("editor-rt"));
    await page.goto("/editor");

    // Import.
    await page.getByRole("button", { name: "Import" }).click();
    await page.getByPlaceholder("Paste it here....").fill(sourceYaml);
    await page.getByRole("button", { name: "Parse it!" }).click();

    // Export — the slideover renders the YAML line-by-line in <pre> blocks.
    await page.getByRole("button", { name: "Export" }).click();
    await page.getByRole("button", { name: "Export as YAML" }).click();

    // Collect the rendered YAML text and parse both sides for comparison.
    const exported = (
      await page.locator("pre").allInnerTexts()
    ).join("\n");
    const parsedOut = YAML.parse(exported);
    const parsedIn = YAML.parse(sourceYaml);

    // Block ids + names + dependency structure must be preserved.
    const norm = (p: any) =>
      (p.blocks ?? [])
        .map((b: any) => ({
          id: b.id,
          name: b.name,
          inputs: (b.inputs ?? []).map((r: any) =>
            typeof r === "string" ? r : `${r.block}:${r.output}`,
          ),
        }))
        .sort((x: any, y: any) => x.id.localeCompare(y.id));

    expect(norm(parsedOut)).toEqual(norm(parsedIn));
    expect(parsedOut.name).toBe(parsedIn.name);
  });

  /**
   * Phase G scenario 2: build a pipeline with an ambiguous connection →
   * resolve via the modal → save.
   *
   * Held as fixme because the trigger is a Vue Flow edge drag between two node
   * handles, which needs deterministic handle coordinates that depend on the
   * canvas layout. To unskip:
   *  1. Seed two blocks whose outputs/inputs are type-ambiguous (≥2 outputs of
   *     the same type matching ≥2 inputs) via the admin block-upload endpoint
   *     (needs an admin user — see e2e/README.md "Seeding").
   *  2. Add both via "Add Block".
   *  3. Drag from the source node's bottom Handle to the target's top Handle
   *     using page.mouse (hover source handle → mouse.down → move to target
   *     handle → mouse.up). Consider adding `data-testid` to the Vue Flow
   *     handles in CustomBlock.vue to make this stable.
   *  4. Assert the ConnectionResolver modal opens, pick the mapping (the modal
   *     shows each input/output `description`), confirm, then Save.
   */
  test.fixme(
    "resolve an ambiguous connection via the modal, then save",
    async () => {},
  );
});
