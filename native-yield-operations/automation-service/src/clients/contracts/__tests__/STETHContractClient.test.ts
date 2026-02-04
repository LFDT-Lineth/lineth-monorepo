import { mock, MockProxy } from "jest-mock-extended";
import type { IBlockchainClient, ILogger } from "@consensys/linea-shared-utils";
import type { PublicClient, TransactionReceipt, Address } from "viem";

import { STETHABI } from "../../../core/abis/STETH.js";

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return {
    ...actual,
    getContract: jest.fn(),
  };
});

import { getContract } from "viem";

const mockedGetContract = getContract as jest.MockedFunction<typeof getContract>;

let STETHContractClient: typeof import("../STETHContractClient.js").STETHContractClient;

beforeAll(async () => {
  ({ STETHContractClient } = await import("../STETHContractClient.js"));
});

// Semantic constants
const TEST_CONTRACT_ADDRESS = "0x1111111111111111111111111111111111111111" as Address;
const ONE_ETH_IN_WEI = 1_000_000_000_000_000_000n;
const ONE_SHARE = 1_000_000_000_000_000_000n;

// Factory function for creating logger mock
const createLoggerMock = (): MockProxy<ILogger> => mock<ILogger>();

describe("STETHContractClient", () => {
  let logger: MockProxy<ILogger>;
  let blockchainClient: MockProxy<IBlockchainClient<PublicClient, TransactionReceipt>>;
  let publicClient: PublicClient;

  const createViemContractStub = () => ({
    abi: STETHABI,
    read: { getPooledEthBySharesRoundUp: jest.fn() },
  });

  beforeEach(() => {
    jest.clearAllMocks();
    logger = createLoggerMock();
    blockchainClient = mock<IBlockchainClient<PublicClient, TransactionReceipt>>();
    publicClient = {} as PublicClient;
    blockchainClient.getBlockchainClient.mockReturnValue(publicClient);
  });

  describe("initialization", () => {
    it("initializes viem contract with correct configuration", () => {
      // Arrange
      const viemContractStub = createViemContractStub();
      mockedGetContract.mockReturnValue(viemContractStub as any);

      // Act
      const client = new STETHContractClient(blockchainClient, TEST_CONTRACT_ADDRESS, logger);

      // Assert
      expect(mockedGetContract).toHaveBeenCalledWith({
        abi: STETHABI,
        address: TEST_CONTRACT_ADDRESS,
        client: publicClient,
      });
      expect(client.getContract()).toBe(viemContractStub);
    });

    it("exposes contract address through getter", () => {
      // Arrange
      const viemContractStub = createViemContractStub();
      mockedGetContract.mockReturnValue(viemContractStub as any);

      // Act
      const client = new STETHContractClient(blockchainClient, TEST_CONTRACT_ADDRESS, logger);

      // Assert
      expect(client.getAddress()).toBe(TEST_CONTRACT_ADDRESS);
    });
  });

  describe("getBalance", () => {
    it("retrieves contract balance from blockchain client", async () => {
      // Arrange
      const viemContractStub = createViemContractStub();
      mockedGetContract.mockReturnValue(viemContractStub as any);
      blockchainClient.getBalance.mockResolvedValue(ONE_ETH_IN_WEI);
      const client = new STETHContractClient(blockchainClient, TEST_CONTRACT_ADDRESS, logger);

      // Act
      const result = await client.getBalance();

      // Assert
      expect(result).toBe(ONE_ETH_IN_WEI);
      expect(blockchainClient.getBalance).toHaveBeenCalledWith(TEST_CONTRACT_ADDRESS);
    });
  });

  describe("getPooledEthBySharesRoundUp", () => {
    it("returns pooled ETH amount when contract call succeeds", async () => {
      // Arrange
      const viemContractStub = createViemContractStub();
      mockedGetContract.mockReturnValue(viemContractStub as any);
      viemContractStub.read.getPooledEthBySharesRoundUp.mockResolvedValue(ONE_ETH_IN_WEI);
      const client = new STETHContractClient(blockchainClient, TEST_CONTRACT_ADDRESS, logger);

      // Act
      const result = await client.getPooledEthBySharesRoundUp(ONE_SHARE);

      // Assert
      expect(result).toBe(ONE_ETH_IN_WEI);
      expect(viemContractStub.read.getPooledEthBySharesRoundUp).toHaveBeenCalledWith([ONE_SHARE]);
    });

    it("returns zero when contract returns null", async () => {
      // Arrange
      const viemContractStub = createViemContractStub();
      mockedGetContract.mockReturnValue(viemContractStub as any);
      viemContractStub.read.getPooledEthBySharesRoundUp.mockResolvedValue(null);
      const client = new STETHContractClient(blockchainClient, TEST_CONTRACT_ADDRESS, logger);

      // Act
      const result = await client.getPooledEthBySharesRoundUp(ONE_SHARE);

      // Assert
      expect(result).toBe(0n);
      expect(viemContractStub.read.getPooledEthBySharesRoundUp).toHaveBeenCalledWith([ONE_SHARE]);
    });

    it("returns undefined and logs error when contract call fails", async () => {
      // Arrange
      const viemContractStub = createViemContractStub();
      mockedGetContract.mockReturnValue(viemContractStub as any);
      const contractError = new Error("Contract call failed");
      viemContractStub.read.getPooledEthBySharesRoundUp.mockRejectedValue(contractError);
      const client = new STETHContractClient(blockchainClient, TEST_CONTRACT_ADDRESS, logger);

      // Act
      const result = await client.getPooledEthBySharesRoundUp(ONE_SHARE);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith(`getPooledEthBySharesRoundUp failed, error=${contractError}`);
    });
  });
});

