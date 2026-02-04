import { mock, MockProxy } from "jest-mock-extended";
import type { ILogger, IBlockchainClient } from "@consensys/linea-shared-utils";
import type { PublicClient, TransactionReceipt, Address, Hex } from "viem";

import { LineaRollupYieldExtensionABI } from "../../../core/abis/LineaRollupYieldExtension.js";

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return {
    ...actual,
    getContract: jest.fn(),
    encodeFunctionData: jest.fn(),
  };
});

import { getContract, encodeFunctionData } from "viem";

const mockedGetContract = getContract as jest.MockedFunction<typeof getContract>;
const mockedEncodeFunctionData = encodeFunctionData as jest.MockedFunction<typeof encodeFunctionData>;

let LineaRollupYieldExtensionContractClient: typeof import("../LineaRollupYieldExtensionContractClient.js").LineaRollupYieldExtensionContractClient;

beforeAll(async () => {
  ({ LineaRollupYieldExtensionContractClient } = await import("../LineaRollupYieldExtensionContractClient.js"));
});

describe("LineaRollupYieldExtensionContractClient", () => {
  // Semantic constants
  const CONTRACT_ADDRESS = "0x1111111111111111111111111111111111111111" as Address;
  const ONE_ETH = 1_000_000_000_000_000_000n;
  const TRANSFER_AMOUNT = 123n;
  const ENCODED_CALLDATA = "0xdeadbeef" as Hex;
  const TX_HASH = "0xhash";

  let logger: MockProxy<ILogger>;
  let blockchainClient: MockProxy<IBlockchainClient<PublicClient, TransactionReceipt>>;
  let publicClient: PublicClient;
  const contractStub = { abi: LineaRollupYieldExtensionABI } as any;

  // Factory function for transaction receipt
  const createTransactionReceipt = (transactionHash: string): TransactionReceipt => ({
    transactionHash,
  }) as unknown as TransactionReceipt;

  beforeEach(() => {
    jest.clearAllMocks();
    logger = mock<ILogger>();
    blockchainClient = mock<IBlockchainClient<PublicClient, TransactionReceipt>>();
    publicClient = {} as PublicClient;
    blockchainClient.getBlockchainClient.mockReturnValue(publicClient);
    mockedGetContract.mockReturnValue(contractStub);
  });

  describe("initialization", () => {
    it("initializes viem contract with provided address and client", () => {
      // Arrange
      // (no additional setup needed)

      // Act
      const client = new LineaRollupYieldExtensionContractClient(logger, blockchainClient, CONTRACT_ADDRESS);

      // Assert
      expect(mockedGetContract).toHaveBeenCalledWith({
        abi: LineaRollupYieldExtensionABI,
        address: CONTRACT_ADDRESS,
        client: publicClient,
      });
      expect(client.getContract()).toBe(contractStub);
    });
  });

  describe("getAddress", () => {
    it("returns the configured contract address", () => {
      // Arrange
      const client = new LineaRollupYieldExtensionContractClient(logger, blockchainClient, CONTRACT_ADDRESS);

      // Act
      const address = client.getAddress();

      // Assert
      expect(address).toBe(CONTRACT_ADDRESS);
    });
  });

  describe("getBalance", () => {
    it("retrieves contract balance from blockchain client", async () => {
      // Arrange
      blockchainClient.getBalance.mockResolvedValue(ONE_ETH);
      const client = new LineaRollupYieldExtensionContractClient(logger, blockchainClient, CONTRACT_ADDRESS);

      // Act
      const balance = await client.getBalance();

      // Assert
      expect(balance).toBe(ONE_ETH);
      expect(blockchainClient.getBalance).toHaveBeenCalledWith(CONTRACT_ADDRESS);
    });
  });

  describe("transferFundsForNativeYield", () => {
    it("encodes function data and sends signed transaction", async () => {
      // Arrange
      const txReceipt = createTransactionReceipt(TX_HASH);
      mockedEncodeFunctionData.mockReturnValue(ENCODED_CALLDATA);
      blockchainClient.sendSignedTransaction.mockResolvedValue(txReceipt);
      const client = new LineaRollupYieldExtensionContractClient(logger, blockchainClient, CONTRACT_ADDRESS);

      // Act
      const receipt = await client.transferFundsForNativeYield(TRANSFER_AMOUNT);

      // Assert
      expect(receipt).toBe(txReceipt);
      expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
        abi: contractStub.abi,
        functionName: "transferFundsForNativeYield",
        args: [TRANSFER_AMOUNT],
      });
      expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(
        CONTRACT_ADDRESS,
        ENCODED_CALLDATA,
        undefined,
        LineaRollupYieldExtensionABI,
      );
    });

    it("logs debug message before transfer", async () => {
      // Arrange
      const txReceipt = createTransactionReceipt(TX_HASH);
      mockedEncodeFunctionData.mockReturnValue(ENCODED_CALLDATA);
      blockchainClient.sendSignedTransaction.mockResolvedValue(txReceipt);
      const client = new LineaRollupYieldExtensionContractClient(logger, blockchainClient, CONTRACT_ADDRESS);

      // Act
      await client.transferFundsForNativeYield(TRANSFER_AMOUNT);

      // Assert
      expect(logger.debug).toHaveBeenCalledWith("transferFundsForNativeYield started, amount=123");
    });

    it("logs info message after successful transfer", async () => {
      // Arrange
      const txReceipt = createTransactionReceipt(TX_HASH);
      mockedEncodeFunctionData.mockReturnValue(ENCODED_CALLDATA);
      blockchainClient.sendSignedTransaction.mockResolvedValue(txReceipt);
      const client = new LineaRollupYieldExtensionContractClient(logger, blockchainClient, CONTRACT_ADDRESS);

      // Act
      await client.transferFundsForNativeYield(TRANSFER_AMOUNT);

      // Assert
      expect(logger.info).toHaveBeenCalledWith("transferFundsForNativeYield succeeded, amount=123, txHash=0xhash");
    });
  });
});
