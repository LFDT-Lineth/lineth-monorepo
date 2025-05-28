const londonOverride = {
  version: "0.8.30",
  settings: {
    evmVersion: "london",
    optimizer: {
      enabled: true,
      runs: 10_000,
    },
  },
};

const londonOverridePaths = [
  "src/messaging/l2/L2MessageService.sol",
  "src/bridging/token/TokenBridge.sol",
  "src/bridging/token/BridgedToken.sol",
  "src/tokens/LineaVoyageXP.sol",
  "src/tokens/LineaSurgeXP.sol",
  "src/tokens/TokenMintingRateLimiter.sol",
  "src/_testing/unit/opcodes/ErrorAndDestructionTesting.sol",
  "src/_testing/unit/opcodes/OpcodeTestContract.sol",
  "src/_testing/unit/opcodes/OpcodeTester.sol",
  "src/_testing/mocks/bridging/MockMessageService.sol",
  "src/_testing/mocks/bridging/MockMessageServiceV2.sol",
  "src/_testing/mocks/bridging/MockTokenBridge.sol",
  "src/_testing/mocks/bridging/TestTokenBridge.sol",
  "src/_testing/mocks/bridging/UpgradedBridgedToken.sol",
  "src/_testing/unit/messaging/TestL2MessageManager.sol",
  "src/_testing/unit/messaging/TestL2MessageService.sol",
];

export const overrides = Object.fromEntries(londonOverridePaths.map((path) => [path, londonOverride]));
