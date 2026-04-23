import type { SpecResult } from "./parse-jest-log";

export interface FlakyTestEntry {
  testName: string;
  specFile: string;
  failures: number;
  appearances: number;
  failRate: number; // 0.00 to 1.00
}

const MIN_APPEARANCES = 3;

export function computeFlakyTests(runResults: { specResults: SpecResult[] }[]): FlakyTestEntry[] {
  const counts = new Map<string, { testName: string; specFile: string; failures: number; appearances: number }>();

  for (const run of runResults) {
    for (const spec of run.specResults) {
      for (const test of spec.tests) {
        if (test.status === "skipped") continue;

        const key = `${spec.specFile}\0${test.name}`;
        let entry = counts.get(key);
        if (!entry) {
          entry = { testName: test.name, specFile: spec.specFile, failures: 0, appearances: 0 };
          counts.set(key, entry);
        }
        entry.appearances++;
        if (test.status === "failed") entry.failures++;
      }
    }
  }

  return [...counts.values()]
    .filter((e) => e.appearances >= MIN_APPEARANCES && e.failures >= 1)
    .map((e) => ({
      testName: e.testName,
      specFile: e.specFile,
      failures: e.failures,
      appearances: e.appearances,
      failRate: parseFloat((e.failures / e.appearances).toFixed(2)),
    }))
    .sort((a, b) => b.failRate - a.failRate || b.failures - a.failures);
}
