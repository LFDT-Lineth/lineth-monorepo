import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: "**/*.spec.ts",
  timeout: 40_000,
  fullyParallel: true,
  projects: [
    {
      name: "unit",
      testMatch: "src/**/*.spec.ts",
    },
  ],
});
