// Shared client for the USFS FIADB-API / EVALIDator endpoint.
//
// https://apps.fs.usda.gov/fiadb-api/
//
// Two quirks drive the design here:
//   1. The service returns HTTP 200 with an HTML error page (not a JSON error)
//      when a request is invalid, so we detect the HTML sentinel and surface the
//      embedded "Received an Error: ..." message as a thrown Error.
//   2. `outputFormat=NJSON` returns the flat/normalized JSON we want; plain
//      `JSON` is the legacy cross-tabulated form and errors on some queries.

export const BASE_URL = "https://apps.fs.usda.gov/fiadb-api";

// A single estimate cell. Grouped queries add GRP1/GRP2/GRP3 label fields.
export interface EstimateRow {
  ESTIMATE: number;
  VARIANCE: number;
  SE: number;
  SE_PERCENT: number;
  PLOT_COUNT: number;
  GRP1?: string;
  GRP2?: string;
  GRP3?: string;
}

export interface ReportMetadata {
  evalGrps?: string[];
  numEstDesc?: string;
  numEvalPlotCount?: number;
  FIAorRPA?: string;
  dbVersion?: string;
  warnings?: string[];
  grmRatioWarning?: boolean;
  [key: string]: unknown;
}

export interface ReportResponse {
  citation: string;
  estimates: EstimateRow[];
  subtotals: Record<string, EstimateRow[]>;
  totals: EstimateRow[];
  metadata: ReportMetadata;
}

// Parameters accepted by the /fullreport endpoint. `wc` and `snum` are required;
// the rest are optional pass-throughs. Grouping params (rselected/cselected/
// pselected) take a LABEL_VAR string from the corresponding parameter dictionary
// (e.g. "County code and name"), NOT the bare DB_VAR.
export interface FullReportParams {
  wc: string;
  snum: string;
  sdenom?: string;
  rselected?: string;
  cselected?: string;
  pselected?: string;
  rtime?: string;
  ctime?: string;
  ptime?: string;
  strFilter?: string;
  FIAorRPA?: string;
  estOnly?: string;
}

// GET query strings can grow past server limits once a long strFilter is
// involved; switch to POST beyond this threshold.
const MAX_GET_QUERY_LENGTH = 1500;

function isErrorPage(body: string): boolean {
  const head = body.slice(0, 512).toLowerCase();
  return head.includes("<!doctype") || head.includes("<html");
}

// Pull the human-readable message out of an EVALIDator HTML error page.
function extractErrorMessage(body: string): string {
  const received = body.match(/Received an Error:\s*([^<\n]+)/i);
  if (received) return received[1].trim();
  const type = body.match(/Error Type:\s*([^<\n]+)/i);
  if (type) return `Error Type: ${type[1].trim()}`;
  return "FIADB-API returned an HTML error page (no message extracted)";
}

function buildParams(
  params: Record<string, string | undefined>,
  outputFormat: string,
): URLSearchParams {
  const usp = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      usp.set(key, String(value));
    }
  }
  usp.set("outputFormat", outputFormat);
  return usp;
}

async function fetchText(
  path: string,
  params: URLSearchParams,
): Promise<string> {
  const query = params.toString();
  let response: Response;
  if (query.length > MAX_GET_QUERY_LENGTH) {
    response = await fetch(`${BASE_URL}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: query,
    });
  } else {
    response = await fetch(`${BASE_URL}${path}?${query}`);
  }

  const body = await response.text();

  if (!response.ok) {
    throw new Error(
      `FIADB-API request failed: HTTP ${response.status} ${response.statusText}`,
    );
  }
  if (isErrorPage(body)) {
    throw new Error(`FIADB-API error: ${extractErrorMessage(body)}`);
  }
  return body;
}

// Run a /fullreport query and return the parsed NJSON response.
export async function fetchFullReport(
  params: FullReportParams,
): Promise<ReportResponse> {
  const usp = buildParams(
    params as Record<string, string | undefined>,
    "NJSON",
  );
  const body = await fetchText("/fullreport", usp);
  let parsed: ReportResponse;
  try {
    parsed = JSON.parse(body) as ReportResponse;
  } catch (err) {
    throw new Error(
      `FIADB-API returned a response that was not valid JSON: ${(err as Error).message}`,
    );
  }
  if (!parsed || !parsed.metadata) {
    throw new Error("FIADB-API response is missing the expected 'metadata' block");
  }
  return parsed;
}

// Fetch a parameter dictionary (e.g. "snum", "wc", "rselected") as JSON.
export async function fetchParameters(name: string): Promise<unknown[]> {
  const usp = new URLSearchParams({ outputFormat: "JSON" });
  const body = await fetchText(
    `/fullreport/parameters/${encodeURIComponent(name)}`,
    usp,
  );
  let parsed: unknown;
  try {
    parsed = JSON.parse(body);
  } catch (err) {
    throw new Error(
      `parameters/${name} returned a response that was not valid JSON: ${(err as Error).message}`,
    );
  }
  if (!Array.isArray(parsed)) {
    throw new Error(`parameters/${name} did not return a JSON array`);
  }
  return parsed;
}
