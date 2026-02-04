import { jest, describe, it, expect, beforeEach, beforeAll } from "@jest/globals";
import { mock, MockProxy } from "jest-mock-extended";
import type { IBlockchainClient, ILogger } from "@consensys/linea-shared-utils";
import type { PublicClient, TransactionReceipt, Address } from "viem";

import { StakingVaultABI } from "../../../core/abis/StakingVault.js";

jest.mock("viem", () => ({
  ...jest.requireActual<typeof import("viem")>("viem"),
  getContract: jest.fn(),
}));

import { getContract } from "viem";

const mockedGetContract = getContract as jest.MockedFunction<typeof getContract>;

let StakingVaultContractClient: typeof import("../StakingVaultContractClient.js").StakingVaultContractClient;

beforeAll(async () => {
  ({ StakingVaultContractClient } = await import("../StakingVaultContractClient.js"));
});

describe("StakingVaultContractClient", () => {
  let blockchainClient: MockProxy<IBlockchainClient<PublicClient, TransactionReceipt>>;
  let logger: MockProxy<ILogger>;
  let publicClient: PublicClient;
  let viemContractStub: any;

  // Semantic constants
  const CONTRACT_ADDRESS = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" as Address;
  const OTHER_CONTRACT_ADDRESS = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" as Address;
  const ONE_ETH = 1_000_000_000_000_000_000n;
  const DEPOSITS_PAUSED = true;
  const DEPOSITS_NOT_PAUSED = false;

  const createViemContractStub = () => ({
    abi: StakingVaultABI,
    read: {
      beaconChainDepositsPaused: jest.fn(),
    },
  });

  beforeEach(() => {
    jest.clearAllMocks();
    blockchainClient = mock<IBlockchainClient<PublicClient, TransactionReceipt>>();
    logger = mock<ILogger>();
    publicClient = {} as PublicClient;
    viemContractStub = createViemContractStub();
    blockchainClient.getBlockchainClient.mockReturnValue(publicClient);
    mockedGetContract.mockReturnValue(viemContractStub);

    // Clear static state before each test
    (StakingVaultContractClient as any).blockchainClient = undefined;
    (StakingVaultContractClient as any).logger = undefined;
    (StakingVaultContractClient as any).clientCache.clear();
  });

  describe("initialize", () => {
    it("sets the static blockchainClient and logger", () => {
      // Arrange
      // (no setup needed)

      // Act
      StakingVaultContractClient.initialize(blockchainClient, logger);

      // Assert
      expect((StakingVaultContractClient as any).blockchainClient).toBe(blockchainClient);
      expect((StakingVaultContractClient as any).logger).toBe(logger);
    });
  });

  describe("constructor", () => {
    it("throws error when blockchainClient is not initialized", () => {
      // Arrange
      // (blockchainClient not initialized)

      // Act & Assert
      expect(() => {
        new StakingVaultContractClient(CONTRACT_ADDRESS);
      }).toThrow(
        "StakingVaultContractClient: blockchainClient must be initialized via StakingVaultContractClient.initialize() before use",
      );
    });

    it("throws error when logger is not initialized", () => {
      // Arrange
      (StakingVaultContractClient as any).blockchainClient = blockchainClient;
      (StakingVaultContractClient as any).logger = undefined;

      // Act & Assert
      expect(() => {
        new StakingVaultContractClient(CONTRACT_ADDRESS);
      }).toThrow(
        "StakingVaultContractClient: logger must be initialized via StakingVaultContractClient.initialize() before use",
      );
    });

    it("creates contract instance with correct configuration", () => {
      // Arrange
      StakingVaultContractClient.initialize(blockchainClient, logger);

      // Act
      const client = new StakingVaultContractClient(CONTRACT_ADDRESS);

      // Assert
      expect(client).toBeInstanceOf(StakingVaultContractClient);
      expect(mockedGetContract).toHaveBeenCalledWith({
        abi: StakingVaultABI,
        address: CONTRACT_ADDRESS,
        client: publicClient,
      });
    });
  });

  describe("getOrCreate", () => {
    it("throws error when blockchainClient is not initialized", () => {
      // Arrange
      // (blockchainClient not initialized)

      // Act & Assert
      expect(() => {
        StakingVaultContractClient.getOrCreate(CONTRACT_ADDRESS);
      }).toThrow(
        "StakingVaultContractClient: blockchainClient must be initialized via StakingVaultContractClient.initialize() before use",
      );
    });

    it("throws error when logger is not initialized", () => {
      // Arrange
      (StakingVaultContractClient as any).blockchainClient = blockchainClient;
      (StakingVaultContractClient as any).logger = undefined;

      // Act & Assert
      expect(() => {
        StakingVaultContractClient.getOrCreate(CONTRACT_ADDRESS);
      }).toThrow(
        "StakingVaultContractClient: logger must be initialized via StakingVaultContractClient.initialize() before use",
      );
    });

    it("creates and caches new client when not cached", () => {
      // Arrange
      StakingVaultContractClient.initialize(blockchainClient, logger);

      // Act
      const client = StakingVaultContractClient.getOrCreate(CONTRACT_ADDRESS);

      // Assert
      expect(client).toBeInstanceOf(StakingVaultContractClient);
      expect(client.getAddress()).toBe(CONTRACT_ADDRESS);
      expect(mockedGetContract).toHaveBeenCalledWith({
        abi: StakingVaultABI,
        address: CONTRACT_ADDRESS,
        client: publicClient,
      });
    });

    it("returns cached client when already exists", () => {
      // Arrange
      StakingVaultContractClient.initialize(blockchainClient, logger);
      const client1 = StakingVaultContractClient.getOrCreate(CONTRACT_ADDRESS);
      mockedGetContract.mockClear();

      // Act
      const client2 = StakingVaultContractClient.getOrCreate(CONTRACT_ADDRESS);

      // Assert
      expect(client1).toBe(client2);
      expect(mockedGetContract).not.toHaveBeenCalled();
    });

    it("creates separate clients for different addresses", () => {
      // Arrange
      StakingVaultContractClient.initialize(blockchainClient, logger);

      // Act
      const client1 = StakingVaultContractClient.getOrCreate(CONTRACT_ADDRESS);
      const client2 = StakingVaultContractClient.getOrCreate(OTHER_CONTRACT_ADDRESS);

      // Assert
      expect(client1).not.toBe(client2);
      expect(client1.getAddress()).toBe(CONTRACT_ADDRESS);
      expect(client2.getAddress()).toBe(OTHER_CONTRACT_ADDRESS);
    });
  });

  describe("getAddress", () => {
    it("returns the contract address", () => {
      // Arrange
      StakingVaultContractClient.initialize(blockchainClient, logger);
      const client = new StakingVaultContractClient(CONTRACT_ADDRESS);

      // Act
      const address = client.getAddress();

      // Assert
      expect(address).toBe(CONTRACT_ADDRESS);
    });
  });

  describe("getContract", () => {
    it("returns the viem contract instance", () => {
      // Arrange
      StakingVaultContractClient.initialize(blockchainClient, logger);
      const client = new StakingVaultContractClient(CONTRACT_ADDRESS);

      // Act
      const contract = client.getContract();

      // Assert
      expect(contract).toBe(viemContractStub);
    });
  });

  describe("getBalance", () => {
    it("returns contract balance from blockchain client", async () => {
      // Arrange
      StakingVaultContractClient.initialize(blockchainClient, logger);
      const client = new StakingVaultContractClient(CONTRACT_ADDRESS);
      blockchainClient.getBalance.mockResolvedValue(ONE_ETH);

      // Act
      const balance = await client.getBalance();

      // Assert
      expect(blockchainClient.getBalance).toHaveBeenCalledWith(CONTRACT_ADDRESS);
      expect(balance).toBe(ONE_ETH);
    });

    it("throws error when blockchainClient is not initialized", async () => {
      // Arrange
      StakingVaultContractClient.initialize(blockchainClient, logger);
      const client = new StakingVaultContractClient(CONTRACT_ADDRESS);
      (StakingVaultContractClient as any).blockchainClient = undefined;

      // Act & Assert
      await expect(client.getBalance()).rejects.toThrow(
        "StakingVaultContractClient: blockchainClient must be initialized via StakingVaultContractClient.initialize() before use",
      );
    });
  });

  describe("beaconChainDepositsPaused", () => {
    it("returns true when deposits are paused", async () => {
      // Arrange
      StakingVaultContractClient.initialize(blockchainClient, logger);
      const client = new StakingVaultContractClient(CONTRACT_ADDRESS);
      viemContractStub.read.beaconChainDepositsPaused.mockResolvedValue(DEPOSITS_PAUSED);

      // Act
      const result = await client.beaconChainDepositsPaused();

      // Assert
      expect(viemContractStub.read.beaconChainDepositsPaused).toHaveBeenCalledTimes(1);
      expect(result).toBe(DEPOSITS_PAUSED);
    });

    it("returns false when deposits are not paused", async () => {
      // Arrange
      StakingVaultContractClient.initialize(blockchainClient, logger);
      const client = new StakingVaultContractClient(CONTRACT_ADDRESS);
      viemContractStub.read.beaconChainDepositsPaused.mockResolvedValue(DEPOSITS_NOT_PAUSED);

      // Act
      const result = await client.beaconChainDepositsPaused();

      // Assert
      expect(viemContractStub.read.beaconChainDepositsPaused).toHaveBeenCalledTimes(1);
      expect(result).toBe(DEPOSITS_NOT_PAUSED);
    });
  });
});
