import { describe, it, expect, beforeAll, beforeEach, jest } from "@jest/globals";

import { createLoggerMock } from "../../../__tests__/helpers/index.js";

// Test constants
const CHAIN_ID_MAINNET = 1;
const CHAIN_ID_HOODI = 2;
const CHAIN_ID_UNSUPPORTED = 999;

const mockExpressApiApplication = jest.fn().mockImplementation(() => ({
  start: jest.fn(),
  stop: jest.fn(),
}));
const mockOperationModeSelector = jest.fn().mockImplementation(() => ({
  start: jest.fn(),
  stop: jest.fn(),
}));
const mockGaugeMetricsPoller = jest.fn().mockImplementation(() => ({
  start: jest.fn().mockImplementation(() => Promise.resolve()),
  stop: jest.fn(),
  poll: jest.fn().mockImplementation(() => Promise.resolve()),
}));
const mockWinstonLogger = jest.fn().mockImplementation(() => createLoggerMock());
const mockViemBlockchainClientAdapter = jest.fn().mockImplementation(() => ({}));
const mockWeb3SignerClientAdapter = jest.fn().mockImplementation(() => ({}));

jest.mock("@consensys/linea-shared-utils", () => ({
  ExponentialBackoffRetryService: jest.fn().mockImplementation(() => ({})),
  ExpressApiApplication: mockExpressApiApplication,
  WinstonLogger: mockWinstonLogger,
  ViemBlockchainClientAdapter: mockViemBlockchainClientAdapter,
  Web3SignerClientAdapter: mockWeb3SignerClientAdapter,
  BeaconNodeApiClient: jest.fn().mockImplementation(() => ({})),
  OAuth2TokenClient: jest.fn().mockImplementation(() => ({})),
}));

jest.mock(
  "../../../services/OperationModeSelector.js",
  () => ({
    OperationModeSelector: mockOperationModeSelector,
  }),
  { virtual: true },
);
jest.mock(
  "../../../services/GaugeMetricsPoller.js",
  () => ({
    GaugeMetricsPoller: mockGaugeMetricsPoller,
  }),
  { virtual: true },
);

jest.mock(
  "../../../services/operation-mode-processors/YieldReportingProcessor.js",
  () => ({
    YieldReportingProcessor: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);
jest.mock(
  "../../../services/operation-mode-processors/OssificationPendingProcessor.js",
  () => ({
    OssificationPendingProcessor: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);
jest.mock(
  "../../../services/operation-mode-processors/OssificationCompleteProcessor.js",
  () => ({
    OssificationCompleteProcessor: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);

jest.mock(
  "../../../clients/contracts/YieldManagerContractClient.js",
  () => ({
    YieldManagerContractClient: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);
jest.mock(
  "../../../clients/contracts/LazyOracleContractClient.js",
  () => ({
    LazyOracleContractClient: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);
jest.mock(
  "../../../clients/contracts/VaultHubContractClient.js",
  () => ({
    VaultHubContractClient: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);
jest.mock(
  "../../../clients/contracts/DashboardContractClient.js",
  () => ({
    DashboardContractClient: {
      initialize: jest.fn(),
    },
  }),
  { virtual: true },
);
jest.mock(
  "../../../clients/contracts/StakingVaultContractClient.js",
  () => ({
    StakingVaultContractClient: {
      initialize: jest.fn(),
    },
  }),
  { virtual: true },
);
jest.mock(
  "../../../clients/contracts/LineaRollupYieldExtensionContractClient.js",
  () => ({
    LineaRollupYieldExtensionContractClient: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);
jest.mock(
  "../../../clients/contracts/STETHContractClient.js",
  () => ({
    STETHContractClient: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);

jest.mock(
  "../../../clients/ConsensysStakingApiClient.js",
  () => ({
    ConsensysStakingApiClient: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);
jest.mock(
  "../../../clients/LidoAccountingReportClient.js",
  () => ({
    LidoAccountingReportClient: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);
jest.mock(
  "../../../clients/BeaconChainStakingClient.js",
  () => ({
    BeaconChainStakingClient: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);

jest.mock(
  "../../metrics/NativeYieldAutomationMetricsService.js",
  () => ({
    NativeYieldAutomationMetricsService: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);
jest.mock(
  "../../metrics/NativeYieldAutomationMetricsUpdater.js",
  () => ({
    NativeYieldAutomationMetricsUpdater: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);
jest.mock(
  "../../../core/services/EstimateGasErrorReporter.js",
  () => ({
    EstimateGasErrorReporter: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);
jest.mock(
  "../../../services/RebalanceQuotaService.js",
  () => ({
    RebalanceQuotaService: jest.fn().mockImplementation(() => ({
      getRebalanceAmountAfterQuota: jest.fn(),
      getStakingDirection: jest.fn(),
    })),
  }),
  { virtual: true },
);
jest.mock(
  "../../metrics/OperationModeMetricsRecorder.js",
  () => ({
    OperationModeMetricsRecorder: jest.fn().mockImplementation(() => ({})),
  }),
  { virtual: true },
);

jest.mock(
  "../../../utils/createApolloClient.js",
  () => ({
    createApolloClient: jest.fn().mockReturnValue({}),
  }),
  { virtual: true },
);

jest.mock("viem/chains", () => ({
  mainnet: { id: 1 },
  hoodi: { id: 2 },
}));

// Factory function for creating bootstrap config
const createBootstrapConfig = () => ({
  dataSources: {
    chainId: CHAIN_ID_MAINNET,
    l1RpcUrl: "https://rpc.example.com",
    beaconChainRpcUrl: "https://beacon.example.com",
    stakingGraphQLUrl: "https://staking.example.com/graphql",
    ipfsBaseUrl: "https://ipfs.example.com",
  },
  consensysStakingOAuth2: {
    tokenEndpoint: "https://auth.example.com/token",
    clientId: "client-id",
    clientSecret: "client-secret",
    audience: "audience",
  },
  contractAddresses: {
    lineaRollupContractAddress: "0x1111111111111111111111111111111111111111",
    lazyOracleAddress: "0x2222222222222222222222222222222222222222",
    vaultHubAddress: "0x3333333333333333333333333333333333333333",
    yieldManagerAddress: "0x4444444444444444444444444444444444444444",
    lidoYieldProviderAddress: "0x5555555555555555555555555555555555555555",
    stethAddress: "0x6666666666666666666666666666666666666666",
    l2YieldRecipientAddress: "0x7777777777777777777777777777777777777777",
  },
  apiPort: 3000,
  timing: {
    trigger: {
      pollIntervalMs: 1000,
      maxInactionMs: 5000,
    },
    contractReadRetryTimeMs: 250,
    gaugeMetricsPollIntervalMs: 5000,
  },
  rebalance: {
    toleranceAmountWei: 5000000000000000000n,
    maxValidatorWithdrawalRequestsPerTransaction: 16,
    minWithdrawalThresholdEth: 42n,
    stakingRebalanceQuotaBps: 1800,
    stakingRebalanceQuotaWindowSizeInCycles: 24,
  },
  reporting: {
    shouldSubmitVaultReport: true,
    shouldReportYield: true,
    isUnpauseStakingEnabled: true,
    minNegativeYieldDiffToReportYieldWei: 1000000000000000000n,
    cyclesPerYieldReport: 12,
  },
  web3signer: {
    url: "https://web3signer.example.com",
    publicKey:
      "0x02aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    keystore: {
      path: "/keystore",
      passphrase: "keystore-pass",
    },
    truststore: {
      path: "/truststore",
      passphrase: "truststore-pass",
    },
    tlsEnabled: true,
  },
  loggerOptions: {
    level: "info",
    transports: [],
  },
});

let NativeYieldAutomationServiceBootstrap: any;

beforeAll(async () => {
  ({ NativeYieldAutomationServiceBootstrap } = await import("../NativeYieldAutomationServiceBootstrap.js"));
});

describe("NativeYieldAutomationServiceBootstrap", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe("startAllServices", () => {
    it("starts all services and logs startup messages", () => {
      // Arrange
      const config = createBootstrapConfig();
      const bootstrap = new NativeYieldAutomationServiceBootstrap(config);

      // Act
      bootstrap.startAllServices();

      // Assert
      const apiInstance = mockExpressApiApplication.mock.results[0]?.value as {
        start: jest.Mock;
        stop: jest.Mock;
      };
      const operationModeSelectorInstance = mockOperationModeSelector.mock.results[0]?.value as {
        start: jest.Mock;
        stop: jest.Mock;
      };
      const gaugeMetricsPollerInstance = mockGaugeMetricsPoller.mock.results[0]?.value as {
        start: jest.Mock;
        stop: jest.Mock;
      };
      const loggerInstance = mockWinstonLogger.mock.results[0]?.value as {
        info: jest.Mock;
      };

      expect(apiInstance.start).toHaveBeenCalledTimes(1);
      expect(gaugeMetricsPollerInstance.start).toHaveBeenCalledTimes(1);
      expect(operationModeSelectorInstance.start).toHaveBeenCalledTimes(1);
      expect(loggerInstance.info).toHaveBeenCalledWith("Metrics API server started");
      expect(loggerInstance.info).toHaveBeenCalledWith("Gauge metrics poller started");
      expect(loggerInstance.info).toHaveBeenCalledWith("Native yield automation service started");
    });
  });

  describe("stopAllServices", () => {
    it("stops all services and logs shutdown messages", () => {
      // Arrange
      const config = createBootstrapConfig();
      const bootstrap = new NativeYieldAutomationServiceBootstrap(config);

      // Act
      bootstrap.stopAllServices();

      // Assert
      const apiInstance = mockExpressApiApplication.mock.results[0]?.value as {
        start: jest.Mock;
        stop: jest.Mock;
      };
      const operationModeSelectorInstance = mockOperationModeSelector.mock.results[0]?.value as {
        start: jest.Mock;
        stop: jest.Mock;
      };
      const gaugeMetricsPollerInstance = mockGaugeMetricsPoller.mock.results[0]?.value as {
        start: jest.Mock;
        stop: jest.Mock;
      };
      const loggerInstance = mockWinstonLogger.mock.results[0]?.value as {
        info: jest.Mock;
      };

      expect(apiInstance.stop).toHaveBeenCalledTimes(1);
      expect(gaugeMetricsPollerInstance.stop).toHaveBeenCalledTimes(1);
      expect(operationModeSelectorInstance.stop).toHaveBeenCalledTimes(1);
      expect(loggerInstance.info).toHaveBeenCalledWith("Metrics API server stopped");
      expect(loggerInstance.info).toHaveBeenCalledWith("Gauge metrics poller stopped");
      expect(loggerInstance.info).toHaveBeenCalledWith("Native yield automation service stopped");
    });
  });

  describe("getConfig", () => {
    it("returns the bootstrap configuration", () => {
      // Arrange
      const config = createBootstrapConfig();
      const bootstrap = new NativeYieldAutomationServiceBootstrap(config);

      // Act
      const result = bootstrap.getConfig();

      // Assert
      expect(result).toBe(config);
    });
  });

  describe("chain configuration", () => {
    it("creates blockchain client with hoodi chain when configured", () => {
      // Arrange
      const config = createBootstrapConfig();
      config.dataSources.chainId = CHAIN_ID_HOODI;

      // Act
      new NativeYieldAutomationServiceBootstrap(config);

      // Assert
      const { hoodi } = jest.requireMock("viem/chains") as { hoodi: { id: number } };
      expect(mockViemBlockchainClientAdapter).toHaveBeenCalledWith(
        expect.anything(), // logger
        config.dataSources.l1RpcUrl,
        hoodi,
        expect.anything(), // contractSignerClient
        expect.anything(), // errorReporter
      );
    });

    it("throws error when configured with unsupported chain id", () => {
      // Arrange
      const config = createBootstrapConfig();
      config.dataSources.chainId = CHAIN_ID_UNSUPPORTED;

      // Act & Assert
      expect(() => new NativeYieldAutomationServiceBootstrap(config)).toThrow(
        `Unsupported chain ID: ${CHAIN_ID_UNSUPPORTED}`,
      );
    });
  });
});
