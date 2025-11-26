import { jest } from "@jest/globals";

const mockExpressApiApplication = jest.fn().mockImplementation(() => ({
  start: jest.fn(),
  stop: jest.fn(),
}));
const mockOperationModeSelector = jest.fn().mockImplementation(() => ({
  start: jest.fn(),
  stop: jest.fn(),
}));
const mockWinstonLogger = jest.fn().mockImplementation(() => ({
  info: jest.fn(),
}));
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
  "../../../clients/contracts/LineaRollupYieldExtensionContractClient.js",
  () => ({
    LineaRollupYieldExtensionContractClient: jest.fn().mockImplementation(() => ({})),
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

let NativeYieldAutomationServiceBootstrap: any;

beforeAll(async () => {
  ({ NativeYieldAutomationServiceBootstrap } = await import("../NativeYieldAutomationServiceBootstrap.js"));
});

const createBootstrapConfig = () => ({
  dataSources: {
    chainId: 1,
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
    l2YieldRecipientAddress: "0x7777777777777777777777777777777777777777",
  },
  apiPort: 3000,
  timing: {
    trigger: {
      pollIntervalMs: 1000,
      maxInactionMs: 5000,
    },
    contractReadRetryTimeMs: 250,
  },
  rebalanceToleranceBps: 500,
  maxValidatorWithdrawalRequestsPerTransaction: 16,
  minWithdrawalThresholdEth: 42n,
  reporting: {
    shouldSubmitVaultReport: true,
    minPositiveYieldToReportWei: 1000000000000000000n,
    minUnpaidLidoProtocolFeesToReportYieldWei: 500000000000000000n,
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

describe("NativeYieldAutomationServiceBootstrap", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("starts services and logs startup messages", () => {
    const config = createBootstrapConfig();

    const bootstrap = new NativeYieldAutomationServiceBootstrap(config);
    bootstrap.startAllServices();

    const apiInstance = mockExpressApiApplication.mock.results[0]?.value as {
      start: jest.Mock;
      stop: jest.Mock;
    };
    const operationModeSelectorInstance = mockOperationModeSelector.mock.results[0]?.value as {
      start: jest.Mock;
      stop: jest.Mock;
    };
    const loggerInstance = mockWinstonLogger.mock.results[0]?.value as {
      info: jest.Mock;
    };

    expect(apiInstance.start).toHaveBeenCalledTimes(1);
    expect(operationModeSelectorInstance.start).toHaveBeenCalledTimes(1);
    expect(loggerInstance.info).toHaveBeenCalledWith("Metrics API server started");
    expect(loggerInstance.info).toHaveBeenCalledWith("Native yield automation service started");
  });

  it("stops services and logs shutdown messages", () => {
    const config = createBootstrapConfig();

    const bootstrap = new NativeYieldAutomationServiceBootstrap(config);
    bootstrap.stopAllServices();

    const apiInstance = mockExpressApiApplication.mock.results[0]?.value as {
      start: jest.Mock;
      stop: jest.Mock;
    };
    const operationModeSelectorInstance = mockOperationModeSelector.mock.results[0]?.value as {
      start: jest.Mock;
      stop: jest.Mock;
    };
    const loggerInstance = mockWinstonLogger.mock.results[0]?.value as {
      info: jest.Mock;
    };

    expect(apiInstance.stop).toHaveBeenCalledTimes(1);
    expect(operationModeSelectorInstance.stop).toHaveBeenCalledTimes(1);
    expect(loggerInstance.info).toHaveBeenCalledWith("Metrics API server stopped");
    expect(loggerInstance.info).toHaveBeenCalledWith("Native yield automation service stopped");
  });

  it("exposes the bootstrap configuration", () => {
    const config = createBootstrapConfig();

    const bootstrap = new NativeYieldAutomationServiceBootstrap(config);

    expect(bootstrap.getConfig()).toBe(config);
  });

  it("creates blockchain client using hoodi chain when configured", () => {
    const config = createBootstrapConfig();
    config.dataSources.chainId = 2;

    new NativeYieldAutomationServiceBootstrap(config);

    const { hoodi } = jest.requireMock("viem/chains") as { hoodi: { id: number } };
    expect(mockViemBlockchainClientAdapter).toHaveBeenCalledWith(
      expect.anything(),
      config.dataSources.l1RpcUrl,
      hoodi,
      expect.anything(),
    );
  });

  it("throws when configured with an unsupported chain id", () => {
    const unsupportedChainId = 999;
    const config = createBootstrapConfig();
    config.dataSources.chainId = unsupportedChainId;

    expect(() => new NativeYieldAutomationServiceBootstrap(config)).toThrow(
      `Unsupported chain ID: ${unsupportedChainId}`,
    );
  });
});
