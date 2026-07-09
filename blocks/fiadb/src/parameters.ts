// Block: fiadb.parameters
//
// Fetches a FIADB-API parameter dictionary (the valid values and metadata for a
// named /fullreport parameter, e.g. "snum", "wc", "rselected") and writes it as
// a JSON array. Useful for discovering estimate-attribute numbers and the
// LABEL_VAR strings that the fullreport block's grouping inputs expect.

import { spadeBlock, JsonFile } from "spade";
import { writeFileSync } from "node:fs";
import { fetchParameters } from "./client.ts";

interface Args {
  name: string;
}

export const handler = spadeBlock({
  inputs: {
    name: "string",
  },
  output: JsonFile,
  description:
    "Fetch a FIADB-API parameter dictionary (e.g. snum, wc, rselected) as a JSON array.",
})(async (args: Args): Promise<JsonFile> => {
  if (!args.name || String(args.name).trim() === "") {
    throw new Error(
      "'name' is required (e.g. snum, wc, rselected, cselected, pselected)",
    );
  }

  const name = String(args.name).trim();
  const dictionary = await fetchParameters(name);

  const outPath = "dictionary.json";
  writeFileSync(outPath, JSON.stringify(dictionary, null, 2));
  console.log(`Wrote ${dictionary.length} entries for parameter '${name}' to ${outPath}`);
  return new JsonFile(outPath);
});
