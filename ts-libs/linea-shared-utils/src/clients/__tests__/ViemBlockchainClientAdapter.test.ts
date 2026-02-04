import {
  Address,
  Chain,
  ContractFunctionRevertedError,
  Hex,
  PublicClient,
  TimeoutError,
  BaseError,
  TransactionReceipt,
  createPublicClient,
  http,
  parseSignature,
  serializeTransaction,
  withTimeout,
  EstimateGasExecutionError,
  RawContractError,
  decodeErrorResult,
} from "viem";
import { sendRawTransaction, waitForTransactionReceipt } from "viem/actions";

import { createLoggerMock } from "../../__tests__/helpers/factories";
import { IContractSignerClient } from "../../core/client/IContractSignerClient";
import { IEstimateGasErrorReporter } from "../../core/services/IEstimateGasErrorReporter";
import { ILogger } from "../../logging/ILogger";
import { ViemBlockchainClientAdapter } from "../ViemBlockchainClientAdapter";

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return {
    ...actual,
    http: jest.fn(() => "mock-transport"),
    createPublicClient: jest.fn(),
    withTimeout: jest.fn((fn: any) => fn({ signal: null })),
    serializeTransaction: jest.fn(),
    parseSignature: jest.fn(),
    decodeErrorResult: jest.fn(),
  };
});

jest.mock("viem/actions", () => ({
  sendRawTransaction: jest.fn(),
  waitForTransactionReceipt: jest.fn(),
}));

const mockedHttp = http as jest.MockedFunction<typeof http>;
const mockedCreatePublicClient = createPublicClient as jest.MockedFunction<typeof createPublicClient>;
const mockedWithTimeout = withTimeout as unknown as jest.MockedFunction<typeof withTimeout>;
const mockedSerializeTransaction = serializeTransaction as jest.MockedFunction<typeof serializeTransaction>;
const mockedParseSignature = parseSignature as jest.MockedFunction<typeof parseSignature>;
const mockedSendRawTransaction = sendRawTransaction as jest.MockedFunction<typeof sendRawTransaction>;
const mockedWaitForTransactionReceipt = waitForTransactionReceipt as jest.MockedFunction<
  typeof waitForTransactionReceipt
>;
const mockedDecodeErrorResult = decodeErrorResult as jest.MockedFunction<typeof decodeErrorResult>;

const createContractSignerClient = (): jest.Mocked<IContractSignerClient> =>
  ({
    getAddress: jest.fn().mockReturnValue("0xSIGNER"),
    sign: jest.fn(),
  }) as unknown as jest.Mocked<IContractSignerClient>;

const createPublicClientMock = () =>
  ({
    getChainId: jest.fn(),
    getBalance: jest.fn(),
    estimateFeesPerGas: jest.fn(),
    getTransactionCount: jest.fn(),
    estimateGas: jest.fn(),
    getTransactionReceipt: jest.fn(),
  }) as unknown as jest.Mocked<PublicClient>;

describe("ViemBlockchainClientAdapter", () => {
  // Test constants
  const RPC_URL = "https://rpc.local";
  const CHAIN_ID = 11155111;
  const CONTRACT_ADDRESS = "0xCONTRACT" as Address;
  const CALLDATA = "0xabcdef" as Hex;
  const SIGNER_ADDRESS = "0xSIGNER";
  const TX_HASH = "0xHASH";
  const SIGNATURE = "0xSIGNATURE";
  const SERIALIZED_TX = "0x02serialized";
  const DEFAULT_MAX_RETRIES = 3;
  const DEFAULT_GAS_RETRY_BUMP_BPS = 1000n;
  const DEFAULT_ATTEMPT_TIMEOUT_MS = 300_000;
  const DEFAULT_GAS_LIMIT_BUFFER_BPS = 1500n;

  const chain = { id: CHAIN_ID } as Chain;

  let logger: jest.Mocked<ILogger>;
  let contractSignerClient: jest.Mocked<IContractSignerClient>;
  let publicClientMock: jest.Mocked<PublicClient>;
  let adapter: ViemBlockchainClientAdapter;

  beforeEach(() => {
    jest.clearAllMocks();

    logger = createLoggerMock({ name: "viem-blockchain-client" });
    contractSignerClient = createContractSignerClient();
    publicClientMock = createPublicClientMock();

    mockedHttp.mockReturnValue("mock-transport" as any);
    mockedCreatePublicClient.mockReturnValue(publicClientMock as unknown as PublicClient);
    mockedWithTimeout.mockImplementation((fn: any, _opts?: any) => fn({ signal: null }));
    mockedParseSignature.mockReturnValue({ r: "0x1", s: "0x2", yParity: 1 } as any);
    mockedSerializeTransaction.mockReturnValue(SERIALIZED_TX);
    mockedSendRawTransaction.mockResolvedValue(TX_HASH);
    mockedWaitForTransactionReceipt.mockResolvedValue({ transactionHash: TX_HASH, status: "success" } as any);

    adapter = new ViemBlockchainClientAdapter(
      logger,
      RPC_URL,
      chain,
      contractSignerClient,
      undefined,
      DEFAULT_MAX_RETRIES,
      DEFAULT_GAS_RETRY_BUMP_BPS,
      DEFAULT_ATTEMPT_TIMEOUT_MS,
    );
  });

  describe("constructor", () => {
    it("throw error when sendTransactionsMaxRetries is less than 1", () => {
      // Arrange
      const invalidMaxRetries = 0;

      // Act & Assert
      expect(
        () =>
          new ViemBlockchainClientAdapter(
            logger,
            RPC_URL,
            chain,
            contractSignerClient,
            undefined,
            invalidMaxRetries,
            DEFAULT_GAS_RETRY_BUMP_BPS,
            DEFAULT_ATTEMPT_TIMEOUT_MS,
          ),
      ).toThrow("sendTransactionsMaxRetries must be at least 1");
    });

    it("configure transport with request and response logging hooks", async () => {
      // Arrange
      expect(mockedHttp).toHaveBeenCalled();
      const transportConfig = mockedHttp.mock.calls[0]?.[1] as {
        onFetchRequest: (request: any) => Promise<void>;
        onFetchResponse: (response: any) => Promise<void>;
      };
      expect(transportConfig).toBeDefined();

      const requestBody = JSON.stringify({ foo: "bar" });
      const requestClone = { text: jest.fn().mockResolvedValue(requestBody) };
      const request = {
        method: "POST",
        url: RPC_URL,
        clone: jest.fn().mockReturnValue(requestClone),
      };

      // Act
      await transportConfig.onFetchRequest(request);

      // Assert
      expect(logger.debug).toHaveBeenCalledWith("onFetchRequest", {
        method: "POST",
        url: RPC_URL,
        body: requestBody,
      });
    });

    it("log warning when request body read fails", async () => {
      // Arrange
      expect(mockedHttp).toHaveBeenCalled();
      const transportConfig = mockedHttp.mock.calls[0]?.[1] as {
        onFetchRequest: (request: any) => Promise<void>;
      };

      const readError = new Error("request-read-fail");
      const failingRequest = {
        clone: jest.fn().mockReturnValue({
          text: jest.fn().mockRejectedValue(readError),
        }),
      };

      // Act
      await transportConfig.onFetchRequest(failingRequest);

      // Assert
      expect(logger.warn).toHaveBeenCalledWith("Failed to read request body", {
        err: readError,
      });
    });

    it("log response body on successful response", async () => {
      // Arrange
      expect(mockedHttp).toHaveBeenCalled();
      const transportConfig = mockedHttp.mock.calls[0]?.[1] as {
        onFetchResponse: (response: any) => Promise<void>;
      };

      const responseBody = JSON.stringify({ ok: true });
      const responseClone = { text: jest.fn().mockResolvedValue(responseBody) };
      const response = {
        status: 200,
        statusText: "OK",
        clone: jest.fn().mockReturnValue(responseClone),
      };

      // Act
      await transportConfig.onFetchResponse(response);

      // Assert
      expect(logger.debug).toHaveBeenCalledWith("onFetchResponse", {
        status: 200,
        statusText: "OK",
        body: responseBody,
      });
    });

    it("log warning when response body read fails", async () => {
      // Arrange
      expect(mockedHttp).toHaveBeenCalled();
      const transportConfig = mockedHttp.mock.calls[0]?.[1] as {
        onFetchResponse: (response: any) => Promise<void>;
      };

      const readError = new Error("read-fail");
      const responseError = {
        clone: jest.fn().mockReturnValue({
          text: jest.fn().mockRejectedValue(readError),
        }),
      };

      // Act
      await transportConfig.onFetchResponse(responseError);

      // Assert
      expect(logger.warn).toHaveBeenCalledWith("Failed to read response body", {
        err: readError,
      });
    });
  });

  describe("getBlockchainClient", () => {
    it("return the underlying public client", () => {
      // Act
      const result = adapter.getBlockchainClient();

      // Assert
      expect(result).toBe(publicClientMock);
    });
  });

  describe("getSignerAddress", () => {
    it("delegate to contract signer client", () => {
      // Act
      const result = adapter.getSignerAddress();

      // Assert
      expect(result).toBe(SIGNER_ADDRESS);
      expect(contractSignerClient.getAddress).toHaveBeenCalledTimes(1);
    });
  });

  describe("getChainId", () => {
    it("delegate to public client", async () => {
      // Arrange
      const expectedChainId = 5;
      publicClientMock.getChainId.mockResolvedValue(expectedChainId);

      // Act
      const result = await adapter.getChainId();

      // Assert
      expect(result).toBe(expectedChainId);
      expect(publicClientMock.getChainId).toHaveBeenCalledTimes(1);
    });
  });

  describe("getBalance", () => {
    it("delegate to public client", async () => {
      // Arrange
      const address = "0xabc" as Address;
      const expectedBalance = 123n;
      publicClientMock.getBalance.mockResolvedValue(expectedBalance);

      // Act
      const result = await adapter.getBalance(address);

      // Assert
      expect(result).toBe(expectedBalance);
      expect(publicClientMock.getBalance).toHaveBeenCalledWith({ address });
    });
  });

  describe("estimateGasFees", () => {
    it("delegate to public client", async () => {
      // Arrange
      const expectedFees = {
        maxFeePerGas: 40n,
        maxPriorityFeePerGas: 3n,
      };
      publicClientMock.estimateFeesPerGas.mockResolvedValue(expectedFees);

      // Act
      const result = await adapter.estimateGasFees();

      // Assert
      expect(result).toEqual(expectedFees);
      expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(1);
    });
  });

  describe("estimateGas error handling", () => {
    it("log enhanced error details when EstimateGasExecutionError with RawContractError is thrown", async () => {
      // Arrange
      const nonce = 1;
      const rawRevertData =
        "0x08c379a00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000a496e73756666696369656e74000000000000000000000000000000000000000000";

      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.getChainId.mockResolvedValue(chain.id);

      class MockEstimateGasExecutionError extends BaseError {
        constructor(message: string, cause?: BaseError) {
          super(message);
          this.name = "EstimateGasExecutionError";
          if (cause) {
            (this as any).cause = cause;
          }
        }
      }
      Object.setPrototypeOf(MockEstimateGasExecutionError.prototype, EstimateGasExecutionError.prototype);

      const rawContractError = Object.assign(new BaseError("execution reverted"), {
        data: rawRevertData,
      }) as RawContractError;

      const estimateGasError = new MockEstimateGasExecutionError("execution reverted", rawContractError);
      estimateGasError.walk = jest.fn().mockReturnValue(rawContractError);

      publicClientMock.estimateGas.mockRejectedValue(estimateGasError);

      // Act & Assert
      await expect(adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toThrow();

      expect(logger.error).toHaveBeenCalledWith("estimateGas failed with enhanced error details", {
        errorType: "EstimateGasExecutionError",
        rawRevertData,
        decodedError: undefined,
        rpcErrorData: expect.any(String),
        originalMessage: expect.stringContaining("execution reverted"),
        contractAddress: CONTRACT_ADDRESS,
        calldata: CALLDATA,
        value: "0",
      });
    });

    it("log enhanced error details when EstimateGasExecutionError with ContractFunctionRevertedError is thrown", async () => {
      // Arrange
      const nonce = 1;
      const rawRevertData =
        "0x08c379a00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000a496e73756666696369656e74000000000000000000000000000000000000000000";

      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.getChainId.mockResolvedValue(chain.id);

      class MockEstimateGasExecutionError extends BaseError {
        constructor(message: string, cause?: BaseError) {
          super(message);
          this.name = "EstimateGasExecutionError";
          if (cause) {
            (this as any).cause = cause;
          }
        }
      }
      Object.setPrototypeOf(MockEstimateGasExecutionError.prototype, EstimateGasExecutionError.prototype);

      const contractError = new ContractFunctionRevertedError({
        abi: [] as any,
        functionName: "test",
        message: "execution reverted",
      });
      (contractError as any).raw = rawRevertData;
      (contractError as any).data = { errorName: "InsufficientFunds", args: [] };
      (contractError as any).reason = "Insufficient funds";

      const estimateGasError = new MockEstimateGasExecutionError("execution reverted", contractError);

      estimateGasError.walk = jest.fn((predicate?: (e: any) => boolean) => {
        if (predicate && predicate(contractError)) {
          return contractError;
        }
        return contractError;
      });

      publicClientMock.estimateGas.mockRejectedValue(estimateGasError);

      // Act & Assert
      await expect(adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toThrow();

      expect(logger.error).toHaveBeenCalledWith("estimateGas failed with enhanced error details", {
        errorType: "EstimateGasExecutionError",
        rawRevertData: undefined,
        decodedError: {
          raw: rawRevertData,
          errorName: "InsufficientFunds",
          args: [],
          reason: "Insufficient funds",
        },
        rpcErrorData: expect.anything(),
        originalMessage: expect.stringContaining("execution reverted"),
        contractAddress: CONTRACT_ADDRESS,
        calldata: CALLDATA,
        value: "0",
      });
    });

    it("log basic error details when non-EstimateGasExecutionError is thrown", async () => {
      // Arrange
      const nonce = 1;
      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.getChainId.mockResolvedValue(chain.id);

      const genericError = new Error("Generic error");
      publicClientMock.estimateGas.mockRejectedValue(genericError);

      // Act & Assert
      await expect(adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toThrow();

      expect(logger.error).toHaveBeenCalledWith("estimateGas failed", {
        error: genericError,
        contractAddress: CONTRACT_ADDRESS,
        calldata: CALLDATA,
        value: "0",
      });
    });

    it("log enhanced error details when EstimateGasExecutionError with RpcRequestError is thrown", async () => {
      // Arrange
      const nonce = 1;
      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.getChainId.mockResolvedValue(chain.id);

      class MockEstimateGasExecutionError extends BaseError {
        constructor(message: string, cause?: BaseError) {
          super(message);
          this.name = "EstimateGasExecutionError";
          if (cause) {
            (this as any).cause = cause;
          }
        }
      }
      Object.setPrototypeOf(MockEstimateGasExecutionError.prototype, EstimateGasExecutionError.prototype);

      class MockRpcRequestError extends BaseError {
        data: { data: string };
        constructor(message: string, data: { data: string }) {
          super(message);
          this.data = data;
        }
      }

      const rpcErrorData = "0x1234";
      const rpcError = new MockRpcRequestError("RPC error", { data: rpcErrorData });

      const estimateGasError = new MockEstimateGasExecutionError("execution reverted", rpcError);

      estimateGasError.walk = jest.fn((predicate?: (e: any) => boolean) => {
        if (predicate && predicate(rpcError)) {
          return rpcError;
        }
        return rpcError;
      });

      publicClientMock.estimateGas.mockRejectedValue(estimateGasError);

      // Act & Assert
      await expect(adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toThrow();

      expect(logger.error).toHaveBeenCalledWith("estimateGas failed with enhanced error details", {
        errorType: "EstimateGasExecutionError",
        rawRevertData: expect.any(String),
        decodedError: undefined,
        rpcErrorData,
        originalMessage: expect.stringContaining("execution reverted"),
        contractAddress: CONTRACT_ADDRESS,
        calldata: CALLDATA,
        value: "0",
      });
    });

    it("manually decode error when automatic decoding fails but ABI is provided", async () => {
      // Arrange
      const nonce = 1;
      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.getChainId.mockResolvedValue(chain.id);

      const mockABI = [
        {
          inputs: [
            { internalType: "uint256", name: "amount", type: "uint256" },
            { internalType: "uint256", name: "withdrawableValue", type: "uint256" },
          ],
          name: "ExceedsWithdrawable",
          type: "error",
        },
      ] as const;

      const rawRevertData =
        "0xf2ed496c000000000000000000000000000000000000000000000025dffc6dedca6c668800000000000000000000000000000000000000000000000ac3b0cfe3a6daf2d1" as Hex;

      class MockEstimateGasExecutionError extends BaseError {
        constructor(message: string, cause?: BaseError) {
          super(message);
          this.name = "EstimateGasExecutionError";
          if (cause) {
            (this as any).cause = cause;
          }
        }
      }
      Object.setPrototypeOf(MockEstimateGasExecutionError.prototype, EstimateGasExecutionError.prototype);

      const rawContractError = Object.assign(new BaseError("execution reverted"), {
        data: rawRevertData,
      }) as RawContractError;

      const estimateGasError = new MockEstimateGasExecutionError("execution reverted", rawContractError);

      estimateGasError.walk = jest.fn().mockReturnValue(rawContractError);

      mockedDecodeErrorResult.mockReturnValue({
        errorName: "ExceedsWithdrawable",
        args: [175921860444160000000000n, 310000000000000000000000n],
      } as any);

      publicClientMock.estimateGas.mockRejectedValue(estimateGasError);

      // Act & Assert
      await expect(adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n, mockABI)).rejects.toThrow();

      expect(mockedDecodeErrorResult).toHaveBeenCalledWith({
        abi: mockABI,
        data: rawRevertData,
      });

      expect(logger.error).toHaveBeenCalledWith("estimateGas failed with enhanced error details", {
        errorType: "EstimateGasExecutionError",
        rawRevertData,
        decodedError: {
          raw: rawRevertData,
          errorName: "ExceedsWithdrawable",
          args: [175921860444160000000000n, 310000000000000000000000n],
          reason: undefined,
        },
        rpcErrorData: expect.anything(),
        originalMessage: expect.stringContaining("execution reverted"),
        contractAddress: CONTRACT_ADDRESS,
        calldata: CALLDATA,
        value: "0",
      });
    });

    it("call errorReporter when estimateGas fails and errorReporter is provided", async () => {
      // Arrange
      const nonce = 1;
      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.getChainId.mockResolvedValue(chain.id);

      const mockABI = [
        {
          inputs: [
            { internalType: "uint256", name: "amount", type: "uint256" },
            { internalType: "uint256", name: "withdrawableValue", type: "uint256" },
          ],
          name: "ExceedsWithdrawable",
          type: "error",
        },
      ] as const;

      const rawRevertData =
        "0xf2ed496c000000000000000000000000000000000000000000000025dffc6dedca6c668800000000000000000000000000000000000000000000000ac3b0cfe3a6daf2d1" as Hex;

      class MockEstimateGasExecutionError extends BaseError {
        constructor(message: string, cause?: BaseError) {
          super(message);
          this.name = "EstimateGasExecutionError";
          if (cause) {
            (this as any).cause = cause;
          }
        }
      }
      Object.setPrototypeOf(MockEstimateGasExecutionError.prototype, EstimateGasExecutionError.prototype);

      const rawContractError = Object.assign(new BaseError("execution reverted"), {
        data: rawRevertData,
      }) as RawContractError;

      const estimateGasError = new MockEstimateGasExecutionError("execution reverted", rawContractError);
      estimateGasError.walk = jest.fn().mockReturnValue(rawContractError);

      mockedDecodeErrorResult.mockReturnValue({
        errorName: "ExceedsWithdrawable",
        args: [175921860444160000000000n, 310000000000000000000000n],
      } as any);

      const errorReporter: IEstimateGasErrorReporter = {
        recordContractError: jest.fn(),
      };

      const adapterWithReporter = new ViemBlockchainClientAdapter(
        logger,
        RPC_URL,
        chain,
        contractSignerClient,
        errorReporter,
        DEFAULT_MAX_RETRIES,
        DEFAULT_GAS_RETRY_BUMP_BPS,
        DEFAULT_ATTEMPT_TIMEOUT_MS,
        DEFAULT_GAS_LIMIT_BUFFER_BPS,
      );

      publicClientMock.estimateGas.mockRejectedValue(estimateGasError);

      // Act & Assert
      await expect(adapterWithReporter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n, mockABI)).rejects.toThrow();

      expect(errorReporter.recordContractError).toHaveBeenCalledWith(
        CONTRACT_ADDRESS,
        rawRevertData,
        "ExceedsWithdrawable",
      );
    });

    it("call errorReporter with undefined errorName when error is not decoded", async () => {
      // Arrange
      const nonce = 1;
      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.getChainId.mockResolvedValue(chain.id);

      const rawRevertData =
        "0xf2ed496c000000000000000000000000000000000000000000000025dffc6dedca6c668800000000000000000000000000000000000000000000000ac3b0cfe3a6daf2d1" as Hex;

      class MockEstimateGasExecutionError extends BaseError {
        constructor(message: string, cause?: BaseError) {
          super(message);
          this.name = "EstimateGasExecutionError";
          if (cause) {
            (this as any).cause = cause;
          }
        }
      }
      Object.setPrototypeOf(MockEstimateGasExecutionError.prototype, EstimateGasExecutionError.prototype);

      const rawContractError = Object.assign(new BaseError("execution reverted"), {
        data: rawRevertData,
      }) as RawContractError;

      const estimateGasError = new MockEstimateGasExecutionError("execution reverted", rawContractError);
      estimateGasError.walk = jest.fn().mockReturnValue(rawContractError);

      mockedDecodeErrorResult.mockImplementation(() => {
        throw new Error("Failed to decode");
      });

      const errorReporter: IEstimateGasErrorReporter = {
        recordContractError: jest.fn(),
      };

      const adapterWithReporter = new ViemBlockchainClientAdapter(
        logger,
        RPC_URL,
        chain,
        contractSignerClient,
        errorReporter,
        DEFAULT_MAX_RETRIES,
        DEFAULT_GAS_RETRY_BUMP_BPS,
        DEFAULT_ATTEMPT_TIMEOUT_MS,
        DEFAULT_GAS_LIMIT_BUFFER_BPS,
      );

      publicClientMock.estimateGas.mockRejectedValue(estimateGasError);

      // Act & Assert
      await expect(adapterWithReporter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toThrow();

      expect(errorReporter.recordContractError).toHaveBeenCalledWith(CONTRACT_ADDRESS, rawRevertData, undefined);
    });
  });

  describe("gas limit buffer", () => {
    it("apply default 15% buffer to estimated gas", async () => {
      // Arrange
      const nonce = 1;
      const estimatedGas = 100_000n;
      const bufferedGas = 115_000n;

      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(estimatedGas);
      publicClientMock.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 100n,
        maxPriorityFeePerGas: 2n,
      });
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      // Act
      await adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

      // Assert
      expect(contractSignerClient.sign).toHaveBeenCalledWith(
        expect.objectContaining({
          gas: bufferedGas,
        }),
      );
      expect(logger.debug).toHaveBeenCalledWith("Gas estimation with buffer applied", {
        originalEstimatedGas: estimatedGas.toString(),
        bufferedGas: bufferedGas.toString(),
        gasLimitBufferBps: DEFAULT_GAS_LIMIT_BUFFER_BPS.toString(),
        contractAddress: CONTRACT_ADDRESS,
      });
    });

    it("apply custom buffer to estimated gas when provided", async () => {
      // Arrange
      const customBufferBps = 2000n;
      const nonce = 1;
      const estimatedGas = 100_000n;
      const bufferedGas = 120_000n;

      const customBufferAdapter = new ViemBlockchainClientAdapter(
        logger,
        RPC_URL,
        chain,
        contractSignerClient,
        undefined,
        DEFAULT_MAX_RETRIES,
        DEFAULT_GAS_RETRY_BUMP_BPS,
        DEFAULT_ATTEMPT_TIMEOUT_MS,
        customBufferBps,
      );

      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(estimatedGas);
      publicClientMock.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 100n,
        maxPriorityFeePerGas: 2n,
      });
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      // Act
      await customBufferAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

      // Assert
      expect(contractSignerClient.sign).toHaveBeenCalledWith(
        expect.objectContaining({
          gas: bufferedGas,
        }),
      );
      expect(logger.debug).toHaveBeenCalledWith("Gas estimation with buffer applied", {
        originalEstimatedGas: estimatedGas.toString(),
        bufferedGas: bufferedGas.toString(),
        gasLimitBufferBps: customBufferBps.toString(),
        contractAddress: CONTRACT_ADDRESS,
      });
    });

    it("apply buffer and then retry multiplier on retries", async () => {
      // Arrange
      const nonce = 1;
      const estimatedGas = 100_000n;
      const firstAttemptGas = 115_000n; // 100_000n * 1.15
      const secondAttemptGas = 126_500n; // 100_000n * 1.15 * 1.1

      const retryableError = Object.assign(new BaseError("Resource unavailable"), { code: -32002 });

      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(estimatedGas);
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      publicClientMock.estimateFeesPerGas
        .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 3n })
        .mockResolvedValueOnce({ maxFeePerGas: 8n, maxPriorityFeePerGas: 2n });
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      mockedWithTimeout
        .mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw retryableError;
        })
        .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

      // Act
      await adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

      // Assert
      expect(contractSignerClient.sign).toHaveBeenNthCalledWith(1, expect.objectContaining({ gas: firstAttemptGas }));
      expect(contractSignerClient.sign).toHaveBeenNthCalledWith(2, expect.objectContaining({ gas: secondAttemptGas }));
    });

    it("handle zero buffer correctly", async () => {
      // Arrange
      const zeroBufferBps = 0n;
      const nonce = 1;
      const estimatedGas = 100_000n;

      const zeroBufferAdapter = new ViemBlockchainClientAdapter(
        logger,
        RPC_URL,
        chain,
        contractSignerClient,
        undefined,
        DEFAULT_MAX_RETRIES,
        DEFAULT_GAS_RETRY_BUMP_BPS,
        DEFAULT_ATTEMPT_TIMEOUT_MS,
        zeroBufferBps,
      );

      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(estimatedGas);
      publicClientMock.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 100n,
        maxPriorityFeePerGas: 2n,
      });
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      // Act
      await zeroBufferAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

      // Assert
      expect(contractSignerClient.sign).toHaveBeenCalledWith(
        expect.objectContaining({
          gas: estimatedGas,
        }),
      );
    });
  });

  describe("getTxReceipt", () => {
    const txHash = "0x1234567890abcdef" as Hex;

    it("return transaction receipt when found", async () => {
      // Arrange
      const mockReceipt = {
        transactionHash: txHash,
        status: "success",
        blockNumber: 12345n,
        gasUsed: 21000n,
      } as TransactionReceipt;

      publicClientMock.getTransactionReceipt.mockResolvedValue(mockReceipt);

      // Act
      const result = await adapter.getTxReceipt(txHash);

      // Assert
      expect(result).toEqual(mockReceipt);
      expect(publicClientMock.getTransactionReceipt).toHaveBeenCalledWith({ hash: txHash });
      expect(publicClientMock.getTransactionReceipt).toHaveBeenCalledTimes(1);
    });

    it("return undefined when transaction is not found", async () => {
      // Arrange
      const notFoundError = Object.assign(new BaseError("Transaction not found"), { code: -32001 });
      publicClientMock.getTransactionReceipt.mockRejectedValue(notFoundError);

      // Act
      const result = await adapter.getTxReceipt(txHash);

      // Assert
      expect(result).toBeUndefined();
      expect(publicClientMock.getTransactionReceipt).toHaveBeenCalledWith({ hash: txHash });
      expect(logger.warn).toHaveBeenCalledWith("getTxReceipt - failed to get transaction receipt", {
        txHash,
        error: notFoundError,
      });
    });

    it("return undefined and log on network error", async () => {
      // Arrange
      const networkError = Object.assign(new BaseError("Network error"), { code: -32603 });
      publicClientMock.getTransactionReceipt.mockRejectedValue(networkError);

      // Act
      const result = await adapter.getTxReceipt(txHash);

      // Assert
      expect(result).toBeUndefined();
      expect(publicClientMock.getTransactionReceipt).toHaveBeenCalledWith({ hash: txHash });
      expect(logger.warn).toHaveBeenCalledWith("getTxReceipt - failed to get transaction receipt", {
        txHash,
        error: networkError,
      });
    });

    it("return undefined and log on any error", async () => {
      // Arrange
      const genericError = new Error("Unexpected error");
      publicClientMock.getTransactionReceipt.mockRejectedValue(genericError);

      // Act
      const result = await adapter.getTxReceipt(txHash);

      // Assert
      expect(result).toBeUndefined();
      expect(publicClientMock.getTransactionReceipt).toHaveBeenCalledWith({ hash: txHash });
      expect(logger.warn).toHaveBeenCalledWith("getTxReceipt - failed to get transaction receipt", {
        txHash,
        error: genericError,
      });
    });
  });

  describe("sendSignedTransaction", () => {
    it("use default constructor parameters and default tx value", async () => {
      // Arrange
      const defaultsAdapter = new ViemBlockchainClientAdapter(logger, RPC_URL, chain, contractSignerClient);

      const nonce = 4;
      const estimatedGas = 200n;
      const bufferedGas = 230n; // 200n * 1.15
      const retryGas = 253n; // 230n * 1.1

      const retryableError = Object.assign(new BaseError("Internal RPC error"), { code: -32603 });

      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(estimatedGas);
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      publicClientMock.estimateFeesPerGas
        .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 3n })
        .mockResolvedValueOnce({ maxFeePerGas: 8n, maxPriorityFeePerGas: 2n });
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      mockedWithTimeout
        .mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw retryableError;
        })
        .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

      // Act
      const receipt = await defaultsAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA);

      // Assert
      expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
      expect(logger.warn).toHaveBeenCalledWith(
        "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=3",
        { error: retryableError },
      );
      expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
      expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      expect(contractSignerClient.sign).toHaveBeenNthCalledWith(1, {
        to: CONTRACT_ADDRESS,
        type: "eip1559",
        data: CALLDATA,
        chainId: chain.id,
        gas: bufferedGas,
        maxFeePerGas: 10n,
        maxPriorityFeePerGas: 3n,
        nonce,
        value: 0n,
      });
      expect(contractSignerClient.sign).toHaveBeenNthCalledWith(2, {
        to: CONTRACT_ADDRESS,
        type: "eip1559",
        data: CALLDATA,
        chainId: chain.id,
        gas: retryGas,
        maxFeePerGas: 11n,
        maxPriorityFeePerGas: 3n,
        nonce,
        value: 0n,
      });
    });

    it("successfully send signed transaction on first attempt", async () => {
      // Arrange
      const nonce = 7;
      const value = 10n;
      const estimatedGas = 21_000n;
      const bufferedGas = 24_150n; // 21_000n * 1.15

      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(estimatedGas);
      publicClientMock.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 100n,
        maxPriorityFeePerGas: 2n,
      });
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      // Act
      const receipt = await adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, value);

      // Assert
      expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
      expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(1);
      expect(contractSignerClient.sign).toHaveBeenCalledWith({
        to: CONTRACT_ADDRESS,
        type: "eip1559",
        data: CALLDATA,
        chainId: chain.id,
        gas: bufferedGas,
        maxFeePerGas: 100n,
        maxPriorityFeePerGas: 2n,
        nonce,
        value,
      });
      expect(mockedParseSignature).toHaveBeenCalledWith(SIGNATURE);
      expect(mockedSerializeTransaction).toHaveBeenCalledWith(
        {
          to: CONTRACT_ADDRESS,
          type: "eip1559",
          data: CALLDATA,
          chainId: chain.id,
          gas: bufferedGas,
          maxFeePerGas: 100n,
          maxPriorityFeePerGas: 2n,
          nonce,
          value,
        },
        { r: "0x1", s: "0x2", yParity: 1 },
      );
      expect(mockedSendRawTransaction).toHaveBeenCalledWith(publicClientMock, {
        serializedTransaction: SERIALIZED_TX,
      });
      expect(mockedWaitForTransactionReceipt).toHaveBeenCalledWith(publicClientMock, { hash: TX_HASH });
    });

    it("pass ABI to estimateGas when provided", async () => {
      // Arrange
      const mockABI = [
        {
          inputs: [],
          name: "TestError",
          type: "error",
        },
      ] as const;

      const nonce = 7;
      const estimatedGas = 21_000n;

      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(estimatedGas);
      publicClientMock.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 100n,
        maxPriorityFeePerGas: 2n,
      });
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      // Act
      const receipt = await adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n, mockABI);

      // Assert
      expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
      expect(publicClientMock.estimateGas).toHaveBeenCalledWith(
        expect.objectContaining({
          abi: mockABI,
          account: SIGNER_ADDRESS,
          to: CONTRACT_ADDRESS,
          data: CALLDATA,
          value: 0n,
        }),
      );
    });

    it("retry on retryable errors and apply gas bump multipliers", async () => {
      // Arrange
      const nonce = 5;
      const estimatedGas = 200n;
      const firstAttemptGas = 230n; // 200n * 1.15
      const secondAttemptGas = 253n; // 230n * 1.1

      const retryableError = Object.assign(new BaseError("Resource unavailable"), { code: -32002 });

      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(estimatedGas);
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      publicClientMock.estimateFeesPerGas
        .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 3n })
        .mockResolvedValueOnce({ maxFeePerGas: 8n, maxPriorityFeePerGas: 2n });
      contractSignerClient.getAddress.mockReturnValue(SIGNER_ADDRESS);
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      mockedWithTimeout
        .mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw retryableError;
        })
        .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

      // Act
      const receipt = await adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

      // Assert
      expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
      expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
      expect(logger.warn).toHaveBeenCalledWith(
        "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=3",
        { error: retryableError },
      );
      expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      expect(contractSignerClient.sign).toHaveBeenNthCalledWith(1, {
        to: CONTRACT_ADDRESS,
        type: "eip1559",
        data: CALLDATA,
        chainId: chain.id,
        gas: firstAttemptGas,
        maxFeePerGas: 10n,
        maxPriorityFeePerGas: 3n,
        nonce,
        value: 0n,
      });
      expect(contractSignerClient.sign).toHaveBeenNthCalledWith(2, {
        to: CONTRACT_ADDRESS,
        type: "eip1559",
        data: CALLDATA,
        chainId: chain.id,
        gas: secondAttemptGas,
        maxFeePerGas: 11n,
        maxPriorityFeePerGas: 3n,
        nonce,
        value: 0n,
      });
    });

    it("not retry when TimeoutError is thrown", async () => {
      // Arrange
      const maxRetries = 2;
      const testAdapter = new ViemBlockchainClientAdapter(
        logger,
        RPC_URL,
        chain,
        contractSignerClient,
        undefined,
        maxRetries,
        DEFAULT_GAS_RETRY_BUMP_BPS,
        DEFAULT_ATTEMPT_TIMEOUT_MS,
      );

      const timeoutError = new TimeoutError({
        body: { message: "timeout" },
        url: "local:test",
      });

      const nonce = 9;
      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(150n);
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      publicClientMock.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 12n,
        maxPriorityFeePerGas: 4n,
      });
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw timeoutError;
      });

      // Act & Assert
      await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(timeoutError);
      expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
        decodedError: expect.any(Error),
      });
      expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
      expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(1);
    });

    it("rethrow ContractFunctionRevertedError without retrying", async () => {
      // Arrange
      const nonce = 1;
      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(50n);
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      publicClientMock.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 5n,
        maxPriorityFeePerGas: 1n,
      });
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      const revertError = new ContractFunctionRevertedError({
        abi: [] as any,
        functionName: "test",
        message: "execution reverted",
      });
      (revertError as any).data = { errorName: "RevertReason" };
      Object.assign(revertError, { code: -32015 });

      mockedWithTimeout.mockReset();
      mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw revertError;
      });

      // Act & Assert
      await expect(adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toThrow();

      expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
        decodedError: expect.any(Error),
      });
      expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    });

    it("rethrow ContractFunctionRevertedError without retrying when error data is missing", async () => {
      // Arrange
      const nonce = 1;
      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(50n);
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      publicClientMock.estimateFeesPerGas.mockResolvedValue({
        maxFeePerGas: 5n,
        maxPriorityFeePerGas: 1n,
      });
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      const revertError = new ContractFunctionRevertedError({
        abi: [] as any,
        functionName: "test",
        message: "execution reverted",
      });
      (revertError as any).data = undefined;
      Object.assign(revertError, { code: -32015 });

      mockedWithTimeout.mockReset();
      mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw revertError;
      });

      // Act & Assert
      await expect(adapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toThrow();
      expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
        decodedError: expect.any(Error),
      });
      expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    });

    it("throw after exhausting retryable error retries", async () => {
      // Arrange
      const maxRetries = 2;
      const testAdapter = new ViemBlockchainClientAdapter(
        logger,
        RPC_URL,
        chain,
        contractSignerClient,
        undefined,
        maxRetries,
        DEFAULT_GAS_RETRY_BUMP_BPS,
        1_000,
      );

      const retryableError = Object.assign(new BaseError("Limit exceeded"), { code: -32005 });

      const nonce = 1;
      publicClientMock.getTransactionCount.mockResolvedValue(nonce);
      publicClientMock.estimateGas.mockResolvedValue(100n);
      publicClientMock.getChainId.mockResolvedValue(chain.id);
      publicClientMock.estimateFeesPerGas
        .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
        .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
      contractSignerClient.sign.mockResolvedValue(SIGNATURE);

      mockedWithTimeout.mockReset();
      mockedWithTimeout.mockImplementation(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw retryableError;
      });

      // Act & Assert
      await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toThrow();

      expect(logger.warn).toHaveBeenCalledTimes(1);
      expect(logger.error).toHaveBeenCalledWith(
        "sendSignedTransaction retry attempts exhausted sendTransactionsMaxRetries=2",
        { error: retryableError },
      );
      expect(mockedWithTimeout).toHaveBeenCalledTimes(maxRetries);
      expect(contractSignerClient.sign).toHaveBeenCalledTimes(maxRetries);
    });

    describe("retry on retryable HTTP status codes", () => {
      it("retry on HTTP status code 408 (Request Timeout)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const httpError = Object.assign(new BaseError("HTTP error"), { status: 408 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw httpError;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error: httpError },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });

      it("retry on HTTP status code 429 (Too Many Requests)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const httpError = Object.assign(new BaseError("HTTP error"), { status: 429 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw httpError;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error: httpError },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });

      it("retry on HTTP status code 500 (Internal Server Error)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const httpError = Object.assign(new BaseError("HTTP error"), { status: 500 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw httpError;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error: httpError },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });

      it("retry on HTTP status code 502 (Bad Gateway)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const httpError = Object.assign(new BaseError("HTTP error"), { status: 502 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw httpError;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error: httpError },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });

      it("retry on HTTP status code 503 (Service Unavailable)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const httpError = Object.assign(new BaseError("HTTP error"), { status: 503 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw httpError;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error: httpError },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });

      it("retry on HTTP status code 504 (Gateway Timeout)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const httpError = Object.assign(new BaseError("HTTP error"), { status: 504 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw httpError;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error: httpError },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });
    });

    describe("do not retry on non-retryable HTTP status codes", () => {
      it("not retry on HTTP status code 400 (Bad Request)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const httpError = Object.assign(new BaseError("HTTP error"), { status: 400 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw httpError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(httpError);
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
        expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(1);
      });
    });

    describe("retry on error name patterns", () => {
      it("retry on WebSocketRequestError", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const wsError = Object.assign(new BaseError("WebSocket error"), { name: "WebSocketRequestError" });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw wsError;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error: wsError },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });

      it("retry on UnknownRpcError", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const unknownRpcError = Object.assign(new BaseError("Unknown RPC error"), { name: "UnknownRpcError" });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw unknownRpcError;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error: unknownRpcError },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });
    });

    describe("retry on default case (error with no code/status/name)", () => {
      it("retry on error without code, status, or matching name properties", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const defaultError = new BaseError("Unknown error");

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw defaultError;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error: defaultError },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });

      it("retry on unknown RPC error code (default retry behavior)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const unknownRpcError = Object.assign(new BaseError("Unknown RPC error"), { code: 9999 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw unknownRpcError;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error: unknownRpcError },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });
    });

    describe("do not retry on capability errors (5700-5760)", () => {
      it("not retry on capability error code 5700 (lower boundary)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const capabilityError = Object.assign(new BaseError("Capability error"), { code: 5700 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw capabilityError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(capabilityError);
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on capability error code 5760 (upper boundary)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const capabilityError = Object.assign(new BaseError("Capability error"), { code: 5760 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw capabilityError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(capabilityError);
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("retry on RPC error code 5699 (just below capability range)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const error = Object.assign(new BaseError("RPC error"), { code: 5699 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw error;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });

      it("retry on RPC error code 5761 (just above capability range)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const error = Object.assign(new BaseError("RPC error"), { code: 5761 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas
          .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
          .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout
          .mockImplementationOnce(async (fn: any, _opts?: any) => {
            await fn({ signal: null });
            throw error;
          })
          .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

        // Act
        const receipt = await testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n);

        // Assert
        expect(receipt).toEqual({ transactionHash: TX_HASH, status: "success" });
        expect(logger.warn).toHaveBeenCalledWith(
          "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
          { error },
        );
        expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
      });
    });

    describe("do not retry on standard RPC errors", () => {
      it("not retry on ParseRpcError (-32700)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const parseError = Object.assign(new BaseError("Parse error"), { code: -32700 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw parseError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(parseError);
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on InvalidRequestRpcError (-32600)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const invalidRequestError = Object.assign(new BaseError("Invalid request"), { code: -32600 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw invalidRequestError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(
          invalidRequestError,
        );
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on MethodNotFoundRpcError (-32601)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const methodNotFoundError = Object.assign(new BaseError("Method not found"), { code: -32601 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw methodNotFoundError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(
          methodNotFoundError,
        );
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on InvalidParamsRpcError (-32602)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const invalidParamsError = Object.assign(new BaseError("Invalid params"), { code: -32602 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw invalidParamsError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(
          invalidParamsError,
        );
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on InvalidInputRpcError (-32000)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const invalidInputError = Object.assign(new BaseError("Invalid input"), { code: -32000 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw invalidInputError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(invalidInputError);
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on ResourceNotFoundRpcError (-32001)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const resourceNotFoundError = Object.assign(new BaseError("Resource not found"), { code: -32001 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw resourceNotFoundError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(
          resourceNotFoundError,
        );
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on TransactionRejectedRpcError (-32003)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const transactionRejectedError = Object.assign(new BaseError("Transaction rejected"), { code: -32003 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw transactionRejectedError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(
          transactionRejectedError,
        );
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on MethodNotSupportedRpcError (-32004)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const methodNotSupportedError = Object.assign(new BaseError("Method not supported"), { code: -32004 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw methodNotSupportedError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(
          methodNotSupportedError,
        );
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on JsonRpcVersionUnsupportedError (-32006)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const jsonRpcVersionError = Object.assign(new BaseError("JSON-RPC version unsupported"), { code: -32006 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw jsonRpcVersionError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(
          jsonRpcVersionError,
        );
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });
    });

    describe("do not retry on provider errors", () => {
      it("not retry on UserRejectedRequestError (4001)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const userRejectedError = Object.assign(new BaseError("User rejected request"), { code: 4001 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw userRejectedError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(userRejectedError);
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on UserRejectedRequestError CAIP-25 (5000)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const userRejectedError = Object.assign(new BaseError("User rejected request"), { code: 5000 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw userRejectedError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(userRejectedError);
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on UnauthorizedProviderError (4100)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const unauthorizedError = Object.assign(new BaseError("Unauthorized provider"), { code: 4100 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw unauthorizedError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(unauthorizedError);
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on UnsupportedProviderMethodError (4200)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const unsupportedMethodError = Object.assign(new BaseError("Unsupported provider method"), { code: 4200 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw unsupportedMethodError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(
          unsupportedMethodError,
        );
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });

      it("not retry on SwitchChainError (4902)", async () => {
        // Arrange
        const maxRetries = 2;
        const testAdapter = new ViemBlockchainClientAdapter(
          logger,
          RPC_URL,
          chain,
          contractSignerClient,
          undefined,
          maxRetries,
          DEFAULT_GAS_RETRY_BUMP_BPS,
          DEFAULT_ATTEMPT_TIMEOUT_MS,
        );

        const switchChainError = Object.assign(new BaseError("Switch chain error"), { code: 4902 });

        const nonce = 1;
        publicClientMock.getTransactionCount.mockResolvedValue(nonce);
        publicClientMock.estimateGas.mockResolvedValue(100n);
        publicClientMock.getChainId.mockResolvedValue(chain.id);
        publicClientMock.estimateFeesPerGas.mockResolvedValue({
          maxFeePerGas: 9n,
          maxPriorityFeePerGas: 1n,
        });
        contractSignerClient.sign.mockResolvedValue(SIGNATURE);

        mockedWithTimeout.mockReset();
        mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
          await fn({ signal: null });
          throw switchChainError;
        });

        // Act & Assert
        await expect(testAdapter.sendSignedTransaction(CONTRACT_ADDRESS, CALLDATA, 0n)).rejects.toBe(switchChainError);
        expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
          decodedError: expect.any(Error),
        });
        expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
        expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
      });
    });
  });
});
