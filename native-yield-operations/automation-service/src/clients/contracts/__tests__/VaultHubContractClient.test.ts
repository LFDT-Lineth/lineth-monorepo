import { mock, MockProxy } from "jest-mock-extended";
import type { IBlockchainClient, ILogger } from "@consensys/linea-shared-utils";
import type { PublicClient, TransactionReceipt, Address } from "viem";
import { VaultHubABI } from "../../../core/abis/VaultHub.js";
import { createLoggerMock } from "../../../__tests__/helpers/index.js";

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return {
    ...actual,
    getContract: jest.fn(),
    parseEventLogs: jest.fn(),
  };
});

import { getContract, parseEventLogs } from "viem";

const mockedGetContract = getContract as jest.MockedFunction<typeof getContract>;
const mockedParseEventLogs = parseEventLogs as jest.MockedFunction<typeof parseEventLogs>;

let VaultHubContractClient: typeof import("../VaultHubContractClient.js").VaultHubContractClient;

beforeAll(async () => {
  ({ VaultHubContractClient } = await import("../VaultHubContractClient.js"));
});

describe("VaultHubContractClient", () => {
  // Test constants
  const VAULT_HUB_ADDRESS = "0x1111111111111111111111111111111111111111" as Address;
  const VAULT_ADDRESS = "0x2222222222222222222222222222222222222222" as Address;
  const OTHER_CONTRACT_ADDRESS = "0x0000000000000000000000000000000000000000" as Address;
  const ONE_ETH = 1_000_000_000_000_000_000n;
  const ETHER_WITHDRAWN_AMOUNT = 123n;
  const LIDO_FEE_AMOUNT = 456n;
  const REPORT_TIMESTAMP = 1704067200n;

  let blockchainClient: MockProxy<IBlockchainClient<PublicClient, TransactionReceipt>>;
  let logger: jest.Mocked<ILogger>;
  let publicClient: PublicClient;
  const viemContractStub = {
    abi: VaultHubABI,
    read: {
      settleableLidoFeesValue: jest.fn(),
      latestReport: jest.fn(),
      isReportFresh: jest.fn(),
      isVaultConnected: jest.fn(),
    },
  } as any;

  // Factory functions
  const createTransactionReceipt = (
    logs: Array<{ address: string; data: string; topics: string[] }>,
  ): TransactionReceipt =>
    ({
      logs,
    }) as unknown as TransactionReceipt;

  const createVaultRebalancedEvent = (address: Address, etherWithdrawn: bigint | undefined) => ({
    eventName: "VaultRebalanced",
    args: { etherWithdrawn },
    address,
  });

  const createLidoFeesSettledEvent = (address: Address, transferred: bigint | undefined) => ({
    eventName: "LidoFeesSettled",
    args: { transferred },
    address,
  });

  const createVaultReport = (timestamp: bigint | null | undefined) => ({
    totalValue: ONE_ETH,
    inOutDelta: 50000000000000000n,
    timestamp,
  });

  beforeEach(() => {
    jest.clearAllMocks();
    blockchainClient = mock<IBlockchainClient<PublicClient, TransactionReceipt>>();
    logger = createLoggerMock();
    publicClient = {} as PublicClient;
    blockchainClient.getBlockchainClient.mockReturnValue(publicClient);
    mockedGetContract.mockReturnValue(viemContractStub);
  });

  const createClient = () => new VaultHubContractClient(blockchainClient, VAULT_HUB_ADDRESS, logger);

  describe("initialization", () => {
    it("initializes viem contract with correct parameters", () => {
      // Arrange
      // (setup in beforeEach)

      // Act
      createClient();

      // Assert
      expect(mockedGetContract).toHaveBeenCalledWith({
        abi: VaultHubABI,
        address: VAULT_HUB_ADDRESS,
        client: publicClient,
      });
    });

    it("exposes contract address through getter", () => {
      // Arrange
      const client = createClient();

      // Act
      const address = client.getAddress();

      // Assert
      expect(address).toBe(VAULT_HUB_ADDRESS);
    });

    it("exposes contract instance through getter", () => {
      // Arrange
      const client = createClient();

      // Act
      const contract = client.getContract();

      // Assert
      expect(contract).toBe(viemContractStub);
    });
  });

  describe("getBalance", () => {
    it("returns contract balance from blockchain client", async () => {
      // Arrange
      blockchainClient.getBalance.mockResolvedValueOnce(ONE_ETH);
      const client = createClient();

      // Act
      const balance = await client.getBalance();

      // Assert
      expect(balance).toBe(ONE_ETH);
      expect(blockchainClient.getBalance).toHaveBeenCalledWith(VAULT_HUB_ADDRESS);
    });
  });

  describe("getLiabilityPaymentFromTxReceipt", () => {
    it("extracts ether withdrawn amount from VaultRebalanced event", () => {
      // Arrange
      const client = createClient();
      const receipt = createTransactionReceipt([
        { address: OTHER_CONTRACT_ADDRESS, data: "0x", topics: [] },
        { address: VAULT_HUB_ADDRESS, data: "0xdata", topics: ["0xtopic"] },
      ]);
      mockedParseEventLogs.mockReturnValueOnce([
        createVaultRebalancedEvent(VAULT_HUB_ADDRESS, ETHER_WITHDRAWN_AMOUNT) as any,
      ]);

      // Act
      const amount = client.getLiabilityPaymentFromTxReceipt(receipt);

      // Assert
      expect(amount).toBe(ETHER_WITHDRAWN_AMOUNT);
      expect(mockedParseEventLogs).toHaveBeenCalledWith({
        abi: viemContractStub.abi,
        eventName: "VaultRebalanced",
        logs: receipt.logs,
      });
    });

    it("returns zero when etherWithdrawn is undefined", () => {
      // Arrange
      const client = createClient();
      const receipt = createTransactionReceipt([{ address: VAULT_HUB_ADDRESS, data: "0xdata", topics: ["0xtopic"] }]);
      mockedParseEventLogs.mockReturnValueOnce([createVaultRebalancedEvent(VAULT_HUB_ADDRESS, undefined) as any]);

      // Act
      const amount = client.getLiabilityPaymentFromTxReceipt(receipt);

      // Assert
      expect(amount).toBe(0n);
      expect(logger.warn).not.toHaveBeenCalled();
    });

    it("returns zero and logs warning when VaultRebalanced event not found", () => {
      // Arrange
      const client = createClient();
      const receipt = createTransactionReceipt([
        { address: VAULT_HUB_ADDRESS.toUpperCase(), data: "0xdead", topics: [] },
      ]);
      mockedParseEventLogs.mockReturnValueOnce([]);

      // Act
      const amount = client.getLiabilityPaymentFromTxReceipt(receipt);

      // Assert
      expect(amount).toBe(0n);
      expect(logger.warn).toHaveBeenCalledWith(
        "getLiabilityPaymentFromTxReceipt - VaultRebalanced event not found in receipt",
      );
    });
  });

  describe("getLidoFeePaymentFromTxReceipt", () => {
    it("extracts transferred amount from LidoFeesSettled event", () => {
      // Arrange
      const client = createClient();
      const receipt = createTransactionReceipt([{ address: VAULT_HUB_ADDRESS, data: "0xfeed", topics: ["0x01"] }]);
      mockedParseEventLogs.mockReturnValueOnce([createLidoFeesSettledEvent(VAULT_HUB_ADDRESS, LIDO_FEE_AMOUNT) as any]);

      // Act
      const amount = client.getLidoFeePaymentFromTxReceipt(receipt);

      // Assert
      expect(amount).toBe(LIDO_FEE_AMOUNT);
      expect(mockedParseEventLogs).toHaveBeenCalledWith({
        abi: viemContractStub.abi,
        eventName: "LidoFeesSettled",
        logs: receipt.logs,
      });
    });

    it("returns zero when transferred is undefined", () => {
      // Arrange
      const client = createClient();
      const receipt = createTransactionReceipt([{ address: VAULT_HUB_ADDRESS, data: "0xfeed", topics: ["0x01"] }]);
      mockedParseEventLogs.mockReturnValueOnce([createLidoFeesSettledEvent(VAULT_HUB_ADDRESS, undefined) as any]);

      // Act
      const amount = client.getLidoFeePaymentFromTxReceipt(receipt);

      // Assert
      expect(amount).toBe(0n);
      expect(logger.warn).not.toHaveBeenCalled();
    });

    it("returns zero when event from different contract address", () => {
      // Arrange
      const client = createClient();
      const receipt = createTransactionReceipt([
        { address: VAULT_ADDRESS, data: "0xaaa", topics: [] },
        { address: VAULT_HUB_ADDRESS, data: "0xbb", topics: [] },
      ]);
      mockedParseEventLogs.mockReturnValueOnce([createLidoFeesSettledEvent(VAULT_ADDRESS, LIDO_FEE_AMOUNT) as any]);

      // Act
      const amount = client.getLidoFeePaymentFromTxReceipt(receipt);

      // Assert
      expect(amount).toBe(0n);
      expect(logger.warn).toHaveBeenCalledWith(
        "getLidoFeePaymentFromTxReceipt - LidoFeesSettled event not found in receipt",
      );
    });
  });

  describe("settleableLidoFeesValue", () => {
    it("returns settleable fees value from contract", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.settleableLidoFeesValue.mockResolvedValueOnce(ONE_ETH);

      // Act
      const result = await client.settleableLidoFeesValue(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(ONE_ETH);
      expect(viemContractStub.read.settleableLidoFeesValue).toHaveBeenCalledWith([VAULT_ADDRESS]);
    });

    it("returns zero when contract returns null", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.settleableLidoFeesValue.mockResolvedValueOnce(null);

      // Act
      const result = await client.settleableLidoFeesValue(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(0n);
    });

    it("returns undefined and logs error when contract call fails", async () => {
      // Arrange
      const client = createClient();
      const error = new Error("Contract call failed");
      viemContractStub.read.settleableLidoFeesValue.mockRejectedValueOnce(error);

      // Act
      const result = await client.settleableLidoFeesValue(VAULT_ADDRESS);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith(`settleableLidoFeesValue failed, error=${error}`);
    });
  });

  describe("getLatestVaultReportTimestamp", () => {
    it("returns timestamp from latest report", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.latestReport.mockResolvedValueOnce(createVaultReport(REPORT_TIMESTAMP));

      // Act
      const result = await client.getLatestVaultReportTimestamp(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(REPORT_TIMESTAMP);
      expect(viemContractStub.read.latestReport).toHaveBeenCalledWith([VAULT_ADDRESS]);
    });

    it("returns zero when timestamp is null", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.latestReport.mockResolvedValueOnce(createVaultReport(null));

      // Act
      const result = await client.getLatestVaultReportTimestamp(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(0n);
    });

    it("returns zero when timestamp is undefined", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.latestReport.mockResolvedValueOnce(createVaultReport(undefined));

      // Act
      const result = await client.getLatestVaultReportTimestamp(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(0n);
    });

    it("returns zero and logs error when contract call fails", async () => {
      // Arrange
      const client = createClient();
      const error = new Error("Contract call failed");
      viemContractStub.read.latestReport.mockRejectedValueOnce(error);

      // Act
      const result = await client.getLatestVaultReportTimestamp(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(0n);
      expect(logger.error).toHaveBeenCalledWith(`getLatestVaultReportTimestamp failed, error=${error}`);
    });
  });

  describe("isReportFresh", () => {
    it("returns true when report is fresh", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.isReportFresh.mockResolvedValueOnce(true);

      // Act
      const result = await client.isReportFresh(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(true);
      expect(viemContractStub.read.isReportFresh).toHaveBeenCalledWith([VAULT_ADDRESS]);
    });

    it("returns false when report is not fresh", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.isReportFresh.mockResolvedValueOnce(false);

      // Act
      const result = await client.isReportFresh(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(false);
    });

    it("returns false when value is null", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.isReportFresh.mockResolvedValueOnce(null);

      // Act
      const result = await client.isReportFresh(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(false);
    });

    it("returns false when value is undefined", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.isReportFresh.mockResolvedValueOnce(undefined);

      // Act
      const result = await client.isReportFresh(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(false);
    });

    it("returns false and logs error when contract call fails", async () => {
      // Arrange
      const client = createClient();
      const error = new Error("Contract call failed");
      viemContractStub.read.isReportFresh.mockRejectedValueOnce(error);

      // Act
      const result = await client.isReportFresh(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(false);
      expect(logger.error).toHaveBeenCalledWith(`isReportFresh failed, error=${error}`);
    });
  });

  describe("isVaultConnected", () => {
    it("returns true when vault is connected", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.isVaultConnected.mockResolvedValueOnce(true);

      // Act
      const result = await client.isVaultConnected(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(true);
      expect(viemContractStub.read.isVaultConnected).toHaveBeenCalledWith([VAULT_ADDRESS]);
    });

    it("returns false when vault is not connected", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.isVaultConnected.mockResolvedValueOnce(false);

      // Act
      const result = await client.isVaultConnected(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(false);
    });

    it("returns false when value is null", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.isVaultConnected.mockResolvedValueOnce(null);

      // Act
      const result = await client.isVaultConnected(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(false);
    });

    it("returns false when value is undefined", async () => {
      // Arrange
      const client = createClient();
      viemContractStub.read.isVaultConnected.mockResolvedValueOnce(undefined);

      // Act
      const result = await client.isVaultConnected(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(false);
    });

    it("returns false and logs error when contract call fails", async () => {
      // Arrange
      const client = createClient();
      const error = new Error("Contract call failed");
      viemContractStub.read.isVaultConnected.mockRejectedValueOnce(error);

      // Act
      const result = await client.isVaultConnected(VAULT_ADDRESS);

      // Assert
      expect(result).toBe(false);
      expect(logger.error).toHaveBeenCalledWith(`isVaultConnected failed, error=${error}`);
    });
  });
});
