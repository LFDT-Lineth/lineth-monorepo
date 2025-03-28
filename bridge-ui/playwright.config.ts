import { defineConfig, devices } from "@playwright/test";
import "dotenv/config";

export default defineConfig({
  testDir: ".",
  testMatch: '**/*.spec.ts',
  // Timeout for tests that don't involve blockchain transactions
  timeout: 40_000,
  fullyParallel: true,
  maxFailures: process.env.CI ? 1 : 0,
  // To consider - cannot really run E2E tests involving blockchain tx in parallel. There is a high risk of reusing the same tx nonce -> leading to dropped transactions
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI
    ? [
      ["html", { open: "never", outputFolder: `playwright-report-${process.env.HEADLESS ? "headless" : "headful"}` }],
      ["list"]
    ]
    : [["html"],["list"]],
  use: {
    baseURL: "http://localhost:3000",
    trace: process.env.CI ? "on" : "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: "pnpm run start",
    url: "http://localhost:3000",
    reuseExistingServer: true,
  },
});
