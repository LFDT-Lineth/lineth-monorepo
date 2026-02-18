import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "src",
  testMatch: "**/*.spec.ts",
  timeout: 30_000,
  fullyParallel: true,
  workers: process.env.CI === "true" ? 1 : undefined,
  reporter: process.env.CI === "true" ? [["list"]] : [["list"]],
});
