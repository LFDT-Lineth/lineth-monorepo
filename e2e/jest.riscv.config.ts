/** @jest-config-loader ts-node */
import config from "./jest.config";

export default {
  ...config,
  // A distinct suffix keeps these tests out of the zkEVM and testnet suites.
  testRegex: "riscv\\.e2e\\.ts$",
  // The execution-only stack has no bridge/liveness prerequisites or background traffic.
  globalSetup: undefined,
  globalTeardown: undefined,
};
