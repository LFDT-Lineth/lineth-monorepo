import { jest, describe, it, expect, beforeAll, beforeEach } from "@jest/globals";
import type { PublicClient, TransactionReceipt, Address } from "viem";
import { DashboardABI } from "../../../core/abis/Dashboard.js";
import { createLoggerMock } from "../../../__tests__/helpers/index.js";

jest.mock("viem", () => ({
  getContract: jest.fn(),
  parseEventLogs: jest.fn(),
}));

import { getContract, parseEventLogs } from "viem";

const mockedGetContract = getContract as jest.MockedFunction<typeof getContract>;
const mockedParseEventLogs = parseEventLogs as jest.MockedFunction<typeof parseEventLogs>;

let DashboardContractClient: typeof import("../DashboardContractClient.js").DashboardContractClient;

beforeAll(async () => {
  ({ DashboardContractClient } = await import("../DashboardContractClient.js"));
});

describe("DashboardContractClient", () => {
  // Test data constants
  const DASHBOARD_ADDRESS = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" as Address;
  const OTHER_CONTRACT_ADDRESS = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" as Address;
  const ZERO_ADDRESS = "0x0000000000000000000000000000000000000000" as Address;
  const ONE_ETH = 1_000_000_000_000_000_000n;
  const NODE_OPERATOR_FEE = 123n;
  const WITHDRAWABLE_AMOUNT = 1000n;
  const TOTAL_VALUE_AMOUNT = 5000n;
  const LIABILITY_SHARES_AMOUNT = 3000n;

  let blockchainClient: any;
  let logger: any;
  let publicClient: PublicClient;
  let viemContractStub: any;

  const createBlockchainClientMock = () => ({
    getBlockchainClient: jest.fn(),
    getBalance: jest.fn(),
  });

  const createViemContractStub = () => ({
    abi: DashboardABI,
    read: {
      obligations: jest.fn(),
      withdrawableValue: jest.fn(),
      totalValue: jest.fn(),
      liabilityShares: jest.fn(),
    },
  });

  const createTransactionReceipt = (
    logs: Array<{ address: string; data: string; topics: string[] }>,
  ): TransactionReceipt =>
    ({
      logs,
    }) as unknown as TransactionReceipt;

  const createFeeDisbursedEvent = (fee: bigint | undefined, address: Address) => ({
    eventName: "FeeDisbursed",
    args: { fee },
    address,
  });

  beforeEach(() => {
    jest.clearAllMocks();
    blockchainClient = createBlockchainClientMock();
    logger = createLoggerMock();
    publicClient = {} as PublicClient;
    viemContractStub = createViemContractStub();
    blockchainClient.getBlockchainClient.mockReturnValue(publicClient);
    mockedGetContract.mockReturnValue(viemContractStub);
    (DashboardContractClient as any).blockchainClient = undefined;
    (DashboardContractClient as any).logger = undefined;
    (DashboardContractClient as any).clientCache.clear();
  });

  const initializeClient = () => {
    if (!(DashboardContractClient as any).logger) {
      DashboardContractClient.initialize(blockchainClient, logger);
    }
    return new DashboardContractClient(DASHBOARD_ADDRESS);
  };

  describe("constructor", () => {
    it("throws error when blockchainClient is not initialized", () => {
      // Arrange
      // (blockchainClient is not initialized)

      // Act & Assert
      expect(() => {
        new DashboardContractClient(DASHBOARD_ADDRESS);
      }).toThrow(
        "DashboardContractClient: blockchainClient must be initialized via DashboardContractClient.initialize() before use",
      );
    });

    it("throws error when logger is not initialized", () => {
      // Arrange
      (DashboardContractClient as any).blockchainClient = blockchainClient;
      (DashboardContractClient as any).logger = undefined;

      // Act & Assert
      expect(() => {
        new DashboardContractClient(DASHBOARD_ADDRESS);
      }).toThrow(
        "DashboardContractClient: logger must be initialized via DashboardContractClient.initialize() before use",
      );
    });

    it("initializes viem contract with correct configuration", () => {
      // Arrange
      DashboardContractClient.initialize(blockchainClient, logger);

      // Act
      const client = new DashboardContractClient(DASHBOARD_ADDRESS);

      // Assert
      expect(client).toBeInstanceOf(DashboardContractClient);
      expect(mockedGetContract).toHaveBeenCalledWith({
        abi: DashboardABI,
        address: DASHBOARD_ADDRESS,
        client: publicClient,
      });
    });
  });

  describe("initialize", () => {
    it("sets static blockchainClient and logger", () => {
      // Arrange & Act
      DashboardContractClient.initialize(blockchainClient, logger);

      // Assert
      expect((DashboardContractClient as any).blockchainClient).toBe(blockchainClient);
      expect((DashboardContractClient as any).logger).toBe(logger);
    });
  });

  describe("getAddress", () => {
    it("returns contract address", () => {
      // Arrange
      const client = initializeClient();

      // Act
      const address = client.getAddress();

      // Assert
      expect(address).toBe(DASHBOARD_ADDRESS);
    });
  });

  describe("getContract", () => {
    it("returns viem contract instance", () => {
      // Arrange
      const client = initializeClient();

      // Act
      const contract = client.getContract();

      // Assert
      expect(contract).toBe(viemContractStub);
    });
  });

  describe("getBalance", () => {
    it("returns contract balance", async () => {
      // Arrange
      blockchainClient.getBalance.mockResolvedValue(ONE_ETH);
      const client = initializeClient();

      // Act
      const balance = await client.getBalance();

      // Assert
      expect(balance).toBe(ONE_ETH);
      expect(blockchainClient.getBalance).toHaveBeenCalledWith(DASHBOARD_ADDRESS);
    });

    it("throws error when blockchainClient is not initialized", async () => {
      // Arrange
      const client = initializeClient();
      (DashboardContractClient as any).blockchainClient = undefined;

      // Act & Assert
      await expect(client.getBalance()).rejects.toThrow(
        "DashboardContractClient: blockchainClient must be initialized via DashboardContractClient.initialize() before use",
      );
    });
  });

  describe("getNodeOperatorFeesPaidFromTxReceipt", () => {
    it("returns node operator fees when FeeDisbursed event is present", () => {
      // Arrange
      const client = initializeClient();
      const receipt = createTransactionReceipt([
        {
          address: ZERO_ADDRESS,
          data: "0x",
          topics: [],
        },
        {
          address: DASHBOARD_ADDRESS,
          data: "0xdata",
          topics: ["0xtopic"],
        },
      ]);
      mockedParseEventLogs.mockReturnValue([createFeeDisbursedEvent(NODE_OPERATOR_FEE, DASHBOARD_ADDRESS) as any]);

      // Act
      const fee = client.getNodeOperatorFeesPaidFromTxReceipt(receipt);

      // Assert
      expect(fee).toBe(NODE_OPERATOR_FEE);
      expect(mockedParseEventLogs).toHaveBeenCalledWith({
        abi: viemContractStub.abi,
        eventName: "FeeDisbursed",
        logs: receipt.logs,
      });
    });

    it("returns zero when FeeDisbursed event has undefined fee", () => {
      // Arrange
      const client = initializeClient();
      const receipt = createTransactionReceipt([
        {
          address: DASHBOARD_ADDRESS,
          data: "0xdata",
          topics: ["0xtopic"],
        },
      ]);
      mockedParseEventLogs.mockReturnValue([createFeeDisbursedEvent(undefined, DASHBOARD_ADDRESS) as any]);

      // Act
      const fee = client.getNodeOperatorFeesPaidFromTxReceipt(receipt);

      // Assert
      expect(fee).toBe(0n);
      expect(logger.warn).not.toHaveBeenCalled();
    });

    it("returns zero when FeeDisbursed event is not found", () => {
      // Arrange
      const client = initializeClient();
      const receipt = createTransactionReceipt([
        {
          address: DASHBOARD_ADDRESS.toUpperCase(),
          data: "0xdead",
          topics: [],
        },
      ]);
      mockedParseEventLogs.mockReturnValue([]);

      // Act
      const fee = client.getNodeOperatorFeesPaidFromTxReceipt(receipt);

      // Assert
      expect(fee).toBe(0n);
      expect(logger.warn).toHaveBeenCalledWith(
        "getNodeOperatorFeesPaidFromTxReceipt - FeeDisbursed event not found in receipt",
      );
    });

    it("returns zero when FeeDisbursed event belongs to different contract", () => {
      // Arrange
      const client = initializeClient();
      const receipt = createTransactionReceipt([
        {
          address: OTHER_CONTRACT_ADDRESS,
          data: "0xaaa",
          topics: [],
        },
        {
          address: DASHBOARD_ADDRESS,
          data: "0xbb",
          topics: [],
        },
      ]);
      mockedParseEventLogs.mockReturnValue([createFeeDisbursedEvent(456n, OTHER_CONTRACT_ADDRESS) as any]);

      // Act
      const fee = client.getNodeOperatorFeesPaidFromTxReceipt(receipt);

      // Assert
      expect(fee).toBe(0n);
      expect(logger.warn).toHaveBeenCalledWith(
        "getNodeOperatorFeesPaidFromTxReceipt - FeeDisbursed event not found in receipt",
      );
    });
  });

  describe("withdrawableValue", () => {
    it("returns withdrawable value from contract", async () => {
      // Arrange
      viemContractStub.read.withdrawableValue.mockResolvedValue(WITHDRAWABLE_AMOUNT);
      const client = initializeClient();

      // Act
      const result = await client.withdrawableValue();

      // Assert
      expect(result).toBe(WITHDRAWABLE_AMOUNT);
      expect(viemContractStub.read.withdrawableValue).toHaveBeenCalledTimes(1);
    });
  });

  describe("totalValue", () => {
    it("returns total value from contract", async () => {
      // Arrange
      viemContractStub.read.totalValue.mockResolvedValue(TOTAL_VALUE_AMOUNT);
      const client = initializeClient();

      // Act
      const result = await client.totalValue();

      // Assert
      expect(result).toBe(TOTAL_VALUE_AMOUNT);
      expect(viemContractStub.read.totalValue).toHaveBeenCalledTimes(1);
    });
  });

  describe("liabilityShares", () => {
    it("returns liability shares from contract", async () => {
      // Arrange
      viemContractStub.read.liabilityShares.mockResolvedValue(LIABILITY_SHARES_AMOUNT);
      const client = initializeClient();

      // Act
      const result = await client.liabilityShares();

      // Assert
      expect(result).toBe(LIABILITY_SHARES_AMOUNT);
      expect(viemContractStub.read.liabilityShares).toHaveBeenCalledTimes(1);
    });
  });

  describe("getOrCreate", () => {
    it("throws error when blockchainClient is not initialized", () => {
      // Arrange
      // (blockchainClient is not initialized)

      // Act & Assert
      expect(() => {
        DashboardContractClient.getOrCreate(DASHBOARD_ADDRESS);
      }).toThrow(
        "DashboardContractClient: blockchainClient must be initialized via DashboardContractClient.initialize() before use",
      );
    });

    it("throws error when logger is not initialized", () => {
      // Arrange
      (DashboardContractClient as any).blockchainClient = blockchainClient;
      (DashboardContractClient as any).logger = undefined;

      // Act & Assert
      expect(() => {
        DashboardContractClient.getOrCreate(DASHBOARD_ADDRESS);
      }).toThrow(
        "DashboardContractClient: logger must be initialized via DashboardContractClient.initialize() before use",
      );
    });

    it("creates and caches new client instance", () => {
      // Arrange
      DashboardContractClient.initialize(blockchainClient, logger);

      // Act
      const client = DashboardContractClient.getOrCreate(DASHBOARD_ADDRESS);

      // Assert
      expect(client).toBeInstanceOf(DashboardContractClient);
      expect(client.getAddress()).toBe(DASHBOARD_ADDRESS);
      expect(mockedGetContract).toHaveBeenCalledWith({
        abi: DashboardABI,
        address: DASHBOARD_ADDRESS,
        client: publicClient,
      });
    });

    it("returns cached client for same address", () => {
      // Arrange
      DashboardContractClient.initialize(blockchainClient, logger);
      const client1 = DashboardContractClient.getOrCreate(DASHBOARD_ADDRESS);
      mockedGetContract.mockClear();

      // Act
      const client2 = DashboardContractClient.getOrCreate(DASHBOARD_ADDRESS);

      // Assert
      expect(client1).toBe(client2);
      expect(mockedGetContract).not.toHaveBeenCalled();
    });

    it("creates separate clients for different addresses", () => {
      // Arrange
      DashboardContractClient.initialize(blockchainClient, logger);

      // Act
      const client1 = DashboardContractClient.getOrCreate(DASHBOARD_ADDRESS);
      const client2 = DashboardContractClient.getOrCreate(OTHER_CONTRACT_ADDRESS);

      // Assert
      expect(client1).not.toBe(client2);
      expect(client1.getAddress()).toBe(DASHBOARD_ADDRESS);
      expect(client2.getAddress()).toBe(OTHER_CONTRACT_ADDRESS);
    });
  });
});
