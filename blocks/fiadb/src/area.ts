// Block: fiadb.area — area of forest (or timberland) land.

import { spadeBlock, File } from "spade";
import { runEstimate, type EstimateArgs } from "./estimate.ts";
import { specFor } from "./attributes.ts";

interface Args extends EstimateArgs {
  land_basis?: string;
}

export const handler = spadeBlock({
  inputs: {
    state: "string",
    year: "string",
    group_by: "string",
    land_basis: "string",
    units: "string",
  },
  output: File,
  description:
    "Area of forest or timberland land for a state (design-based FIA estimate + sampling error), optionally grouped, in imperial or SI units.",
})((args: Args): Promise<File> => runEstimate(specFor("area", args.land_basis), args));
