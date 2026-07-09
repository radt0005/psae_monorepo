// Dispatcher for the fiadb collection. The compiled binary is invoked as
// `fiadb <block>` (core.ResolveEntrypoint), so we route argv[2] to the matching
// block handler and let the spade runtime read params/inputs and write outputs.

import { run } from "spade";
import { handler as fullreport } from "./fullreport.ts";
import { handler as parameters } from "./parameters.ts";
import { handler as area } from "./area.ts";
import { handler as volume } from "./volume.ts";
import { handler as biomass } from "./biomass.ts";
import { handler as carbon } from "./carbon.ts";

const handlers: Record<string, Function> = {
  fullreport,
  parameters,
  area,
  volume,
  biomass,
  carbon,
};

const block = process.argv[2];
const selected = block ? handlers[block] : undefined;

if (!selected) {
  console.error(`unknown block: ${block ?? "(none)"}`);
  console.error(`available blocks: ${Object.keys(handlers).join(", ")}`);
  process.exit(1);
}

await run(selected);
