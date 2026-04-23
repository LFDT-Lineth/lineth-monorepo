import { computeFlakyTests } from "./compute-flaky-tests";

import type { SpecResult } from "./parse-jest-log";

// Minimal run shape needed by computeFlakyTests
interface MinimalRunResult {
  specResults: SpecResult[];
}

describe("computeFlakyTests", () => {
  it("returns empty array when no tests have failures", () => {
    const runs: MinimalRunResult[] = [
      {
        specResults: [
          {
            specFile: "src/a.spec.ts",
            status: "PASS",
            durationSeconds: 10,
            tests: [{ name: "test1", durationMs: 100, status: "passed" }],
          },
        ],
      },
      {
        specResults: [
          {
            specFile: "src/a.spec.ts",
            status: "PASS",
            durationSeconds: 10,
            tests: [{ name: "test1", durationMs: 100, status: "passed" }],
          },
        ],
      },
      {
        specResults: [
          {
            specFile: "src/a.spec.ts",
            status: "PASS",
            durationSeconds: 10,
            tests: [{ name: "test1", durationMs: 100, status: "passed" }],
          },
        ],
      },
    ];
    expect(computeFlakyTests(runs)).toEqual([]);
  });

  it("excludes tests with fewer than MIN_APPEARANCES (3)", () => {
    const runs: MinimalRunResult[] = [
      {
        specResults: [
          {
            specFile: "src/a.spec.ts",
            status: "FAIL",
            durationSeconds: 10,
            tests: [{ name: "test1", durationMs: 100, status: "failed" }],
          },
        ],
      },
      {
        specResults: [
          {
            specFile: "src/a.spec.ts",
            status: "FAIL",
            durationSeconds: 10,
            tests: [{ name: "test1", durationMs: 100, status: "failed" }],
          },
        ],
      },
    ];
    expect(computeFlakyTests(runs)).toEqual([]);
  });

  it("excludes skipped test appearances from counts", () => {
    const runs: MinimalRunResult[] = [
      {
        specResults: [
          {
            specFile: "src/a.spec.ts",
            status: "PASS",
            durationSeconds: 10,
            tests: [{ name: "test1", durationMs: 0, status: "skipped" }],
          },
        ],
      },
      {
        specResults: [
          {
            specFile: "src/a.spec.ts",
            status: "PASS",
            durationSeconds: 10,
            tests: [{ name: "test1", durationMs: 0, status: "skipped" }],
          },
        ],
      },
      {
        specResults: [
          {
            specFile: "src/a.spec.ts",
            status: "PASS",
            durationSeconds: 10,
            tests: [{ name: "test1", durationMs: 0, status: "skipped" }],
          },
        ],
      },
    ];
    expect(computeFlakyTests(runs)).toEqual([]);
  });

  it("ranks tests by failure rate descending, then by failures descending", () => {
    const runs: MinimalRunResult[] = Array.from({ length: 5 }, () => ({
      specResults: [
        {
          specFile: "src/a.spec.ts",
          status: "PASS" as const,
          durationSeconds: 10,
          tests: [
            { name: "stable", durationMs: 100, status: "passed" as const },
            { name: "flaky", durationMs: 100, status: "passed" as const },
          ],
        },
        {
          specFile: "src/b.spec.ts",
          status: "PASS" as const,
          durationSeconds: 10,
          tests: [{ name: "very-flaky", durationMs: 100, status: "passed" as const }],
        },
      ],
    }));
    // Make "flaky" fail 2/5 = 40%
    runs[0].specResults[0].tests[1] = { name: "flaky", durationMs: 100, status: "failed" };
    runs[2].specResults[0].tests[1] = { name: "flaky", durationMs: 100, status: "failed" };
    // Make "very-flaky" fail 3/5 = 60%
    runs[0].specResults[1].tests[0] = { name: "very-flaky", durationMs: 100, status: "failed" };
    runs[1].specResults[1].tests[0] = { name: "very-flaky", durationMs: 100, status: "failed" };
    runs[3].specResults[1].tests[0] = { name: "very-flaky", durationMs: 100, status: "failed" };

    const result = computeFlakyTests(runs);

    expect(result).toHaveLength(2);
    expect(result[0]).toEqual({
      testName: "very-flaky",
      specFile: "src/b.spec.ts",
      failures: 3,
      appearances: 5,
      failRate: 0.6,
    });
    expect(result[1]).toEqual({
      testName: "flaky",
      specFile: "src/a.spec.ts",
      failures: 2,
      appearances: 5,
      failRate: 0.4,
    });
  });

  it("groups by (specFile, testName) so same test name in different specs is separate", () => {
    const runs: MinimalRunResult[] = Array.from({ length: 3 }, () => ({
      specResults: [
        {
          specFile: "src/a.spec.ts",
          status: "FAIL" as const,
          durationSeconds: 10,
          tests: [{ name: "shared-name", durationMs: 100, status: "failed" as const }],
        },
        {
          specFile: "src/b.spec.ts",
          status: "PASS" as const,
          durationSeconds: 10,
          tests: [{ name: "shared-name", durationMs: 100, status: "passed" as const }],
        },
      ],
    }));

    const result = computeFlakyTests(runs);

    expect(result).toHaveLength(1);
    expect(result[0].specFile).toBe("src/a.spec.ts");
    expect(result[0].failures).toBe(3);
  });
});
