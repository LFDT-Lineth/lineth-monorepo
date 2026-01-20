const useViaIR = process.env.ENABLE_VIA_IR === "true";

const lineaOverride = {
  version: "0.8.33",
  settings: {
    viaIR: useViaIR,
    optimizer: {
      enabled: true,
      runs: 10_000,
    },
    evmVersion: "osaka",
  },
};

const lineaOverridePaths = [
  "src/messaging/l2/L2MessageService.sol",
  "src/operational/LineaSequencerUptimeFeed.sol",
  "src/bridging/token/TokenBridge.sol",
  "src/bridging/token/BridgedToken.sol",
  "src/tokens/LineaVoyageXP.sol",
  "src/tokens/LineaSurgeXP.sol",
  "src/tokens/TokenMintingRateLimiter.sol",
  "src/_testing/mocks/bridging/MockMessageService.sol",
  "src/_testing/mocks/bridging/MockMessageServiceV2.sol",
  "src/_testing/mocks/bridging/MockTokenBridge.sol",
  "src/_testing/mocks/bridging/TestTokenBridge.sol",
  "src/_testing/mocks/bridging/UpgradedBridgedToken.sol",
  "src/_testing/unit/messaging/TestL2MessageManager.sol",
  "src/_testing/unit/messaging/TestL2MessageService.sol",
  "src/predeploy/UpgradeableConsolidationQueuePredeploy.sol",
  "src/predeploy/UpgradeableWithdrawalQueuePredeploy.sol",
  "src/predeploy/UpgradeableBeaconChainDepositPredeploy.sol",
  "src/operational/RollupRevenueVault.sol",
];

export const overrides = {
  ...Object.fromEntries(lineaOverridePaths.map((path) => [path, lineaOverride])),
  "src/yield/YieldManager.sol": {
    version: "0.8.33",
    settings: {
      viaIR: useViaIR,
      optimizer: {
        enabled: true,
        runs: 1500,
      },
      evmVersion: "osaka",
    },
  },
  "src/_testing/unit/yield/TestYieldManager.sol": {
    version: "0.8.33",
    settings: {
      viaIR: useViaIR,
      optimizer: {
        enabled: true,
        runs: 10,
      },
      evmVersion: "osaka",
    },
  },
  "src/_testing/unit/rollup/TestLineaRollup.sol": {
    version: "0.8.33",
    settings: {
      viaIR: useViaIR,
      optimizer: {
        enabled: true,
        runs: 1000,
      },
      evmVersion: "osaka",
    },
  },
  "src/rollup/LineaRollup.sol": {
    version: "0.8.33",
    settings: {
      viaIR: useViaIR,
      optimizer: {
        enabled: true,
        runs: 7500,
      },
      evmVersion: "osaka",
    },
  },
};
