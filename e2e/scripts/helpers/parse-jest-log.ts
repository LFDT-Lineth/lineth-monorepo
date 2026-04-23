// Parses raw Jest output from the E2E CI job into per-spec results.
// Used by `e2e/scripts/generate-e2e-runtime-report.ts` when building the runtime report.
export type JobConclusion = "success" | "failure";
export type SpecStatus = "PASS" | "FAIL" | "TIMEOUT";
export type TestStatus = "passed" | "failed" | "skipped";

export interface TestResult {
  name: string;
  durationMs: number;
  status: TestStatus;
}

export interface SpecResult {
  specFile: string;
  status: SpecStatus;
  durationSeconds: number;
  tests: TestResult[];
}

const SPEC_FILE_RE = /^(PASS|FAIL)\s+(src\/[^/]+\.spec\.ts)(?:\s+\(([0-9.]+)\s*s\))?/;
// Matches any PASS/FAIL spec line regardless of path depth - used to detect non-top-level
// specs so we can null out currentSpec and avoid leaking their tests into the previous spec.
const ANY_SPEC_LINE_RE = /^(PASS|FAIL)\s+src\//;
const TEST_CASE_RE = /^\s+(✓|✕|×)\s+(.+?)\s+\((\d+)\s*ms\)/;
const SKIPPED_TEST_RE = /^\s+○\s+skipped\s+(.+)/;
const GHA_TIMESTAMP_RE = /^\d{4}-\d{2}-\d{2}T[\d:.]+Z\s/;
const JEST_SUMMARY_RE = /^Test Suites:\s+(?:(\d+) failed,\s+)?(?:(\d+) passed,\s+)?(\d+) total/;

export function parseJestLog(
  rawLog: string,
  jobConclusion: JobConclusion,
  timeoutEligibleSpecFiles: readonly string[] = [],
): SpecResult[] {
  const lines = rawLog.split("\n");
  const specResults: SpecResult[] = [];
  let currentSpec: SpecResult | null = null;

  for (const rawLine of lines) {
    const line = rawLine.replace(GHA_TIMESTAMP_RE, "");

    const specMatch = line.match(SPEC_FILE_RE);
    if (specMatch) {
      currentSpec = {
        specFile: specMatch[2],
        status: specMatch[1] as "PASS" | "FAIL",
        durationSeconds: specMatch[3] !== undefined ? parseFloat(specMatch[3]) : 0,
        tests: [],
      };
      specResults.push(currentSpec);
      continue;
    }

    if (ANY_SPEC_LINE_RE.test(line)) {
      currentSpec = null;
      continue;
    }

    if (!currentSpec) continue;

    const testMatch = line.match(TEST_CASE_RE);
    if (testMatch) {
      currentSpec.tests.push({
        name: testMatch[2],
        durationMs: parseInt(testMatch[3], 10),
        status: testMatch[1] === "✓" ? "passed" : "failed",
      });
      continue;
    }

    const skippedMatch = line.match(SKIPPED_TEST_RE);
    if (skippedMatch) {
      currentSpec.tests.push({
        name: skippedMatch[1],
        durationMs: 0,
        status: "skipped",
      });
    }
  }

  const parsedFiles = new Set(specResults.map((result) => result.specFile));

  if (jobConclusion === "failure" && parsedFiles.size > 0) {
    for (const specFile of timeoutEligibleSpecFiles) {
      if (!parsedFiles.has(specFile)) {
        specResults.push({
          specFile,
          status: "TIMEOUT",
          durationSeconds: NaN,
          tests: [],
        });
      }
    }
  }

  for (const rawLine of lines) {
    const line = rawLine.replace(GHA_TIMESTAMP_RE, "");
    const summaryMatch = line.match(JEST_SUMMARY_RE);
    if (!summaryMatch) continue;

    const expectedTotal = parseInt(summaryMatch[3], 10);
    const actualParsed = specResults.filter((result) => result.status !== "TIMEOUT").length;

    if (actualParsed !== expectedTotal) {
      console.warn(
        `Warning: Jest reported ${expectedTotal} test suites but parser found ${actualParsed} spec results (some may be helper tests excluded by filter).`,
      );
    }

    break;
  }

  return specResults;
}
