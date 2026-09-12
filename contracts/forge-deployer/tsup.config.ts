import { defineConfig } from "tsup";

export default defineConfig({
  entry: {
    cli: "src/cli.ts",
    "steps/l1-rollup": "src/steps/l1-rollup.ts",
    "steps/l2-message-service": "src/steps/l2-message-service.ts",
    "steps/token-bridge": "src/steps/token-bridge.ts",
    "steps/deterministic-deployment-proxy": "src/steps/deterministic-deployment-proxy.ts",
    "steps/bootstrap": "src/steps/bootstrap.ts",
  },
  format: ["cjs"],
  platform: "node",
  target: "node24",
  outDir: "dist",
  clean: true,
  sourcemap: true,
  noExternal: ["dotenv", "ethers"],
});
