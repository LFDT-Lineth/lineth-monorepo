import { defineConfig, devices, ReporterDescription } from "@playwright/test";
import "dotenv/config";

const isUnit = process.env.UNIT === "true";

type LiteralUnion<T extends U, U = string> = T | (U & { zz_IGNORE_ME?: never });

function getReporter(opts: {
  isUnitTest: boolean;
  isCI: boolean;
}):
  | LiteralUnion<"html" | "list" | "dot" | "line" | "github" | "json" | "junit" | "null", string>
  | ReporterDescription[]
  | undefined {
  const { isUnitTest, isCI } = opts;

  if (isUnitTest) {
    return undefined;
  }
  if (isCI) {
    return [
      ["html", { open: "never", outputFolder: `playwright-report-${process.env.HEADLESS ? "headless" : "headful"}` }],
      ["list"],
    ];
  }
  return [["html"], ["list"]];
}

export default defineConfig({
  testDir: ".",
  testMatch: "**/*.spec.ts",
  // Timeout for tests that don't involve blockchain transactions
  timeout: 40_000,
  fullyParallel: true,
  maxFailures: 1,
  // To consider - cannot really run E2E tests involving blockchain tx in parallel. There is a high risk of reusing the same tx nonce -> leading to dropped transactions
  workers: process.env.CI === "true" ? 1 : undefined,
  reporter: getReporter({ isUnitTest: isUnit, isCI: process.env.CI === "true" }),
  use: {
    baseURL: "http://localhost:3000",
    trace: process.env.CI === "true" ? "on" : "retain-on-failure",
  },
  projects: [
    {
      name: "setup",
      testMatch: /global\.setup\.ts/,
    },
    {
      name: "chromium",
      testMatch: "test/**/*.spec.ts",
      use: { ...devices["Desktop Chrome"] },
      dependencies: ["setup"],
    },
    {
      name: "unit",
      testMatch: "src/**/*.spec.ts",
    },
  ],
  webServer: !isUnit
    ? {
        command: "pnpm run start",
        url: "http://localhost:3000",
        reuseExistingServer: true,
      }
    : undefined,
});
