import {
  Address,
  Chain,
  ContractFunctionRevertedError,
  Hex,
  PublicClient,
  TimeoutError,
  BaseError,
  createPublicClient,
  http,
  parseSignature,
  serializeTransaction,
  withTimeout,
} from "viem";
import { sendRawTransaction, waitForTransactionReceipt } from "viem/actions";

import { IContractSignerClient } from "../../core/client/IContractSignerClient";
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

const createLogger = (): jest.Mocked<ILogger> =>
  ({
    name: "viem-blockchain-client",
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
  }) as jest.Mocked<ILogger>;

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
  }) as unknown as jest.Mocked<PublicClient>;

describe("ViemBlockchainClientAdapter", () => {
  const rpcUrl = "https://rpc.local";
  const chain = { id: 11155111 } as Chain;
  const contractAddress = "0xCONTRACT" as Address;
  const calldata = "0xabcdef" as Hex;

  let logger: jest.Mocked<ILogger>;
  let contractSignerClient: jest.Mocked<IContractSignerClient>;
  let publicClientMock: jest.Mocked<PublicClient>;
  let adapter: ViemBlockchainClientAdapter;

  beforeEach(() => {
    jest.clearAllMocks();
    logger = createLogger();
    contractSignerClient = createContractSignerClient();
    publicClientMock = createPublicClientMock();
    mockedHttp.mockReturnValue("mock-transport" as any);
    mockedCreatePublicClient.mockReturnValue(publicClientMock as unknown as PublicClient);
    mockedWithTimeout.mockImplementation((fn: any, _opts?: any) => fn({ signal: null }));
    mockedParseSignature.mockReturnValue({ r: "0x1", s: "0x2", yParity: 1 } as any);
    mockedSerializeTransaction.mockReturnValue("0x02serialized");
    mockedSendRawTransaction.mockResolvedValue("0xHASH");
    mockedWaitForTransactionReceipt.mockResolvedValue({ transactionHash: "0xHASH", status: "success" } as any);

    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 3, 1000n, 300_000);
  });

  it("logs request and response bodies in viem transport hooks", async () => {
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
      url: "https://rpc.local",
      clone: jest.fn().mockReturnValue(requestClone),
    };

    await transportConfig.onFetchRequest(request);

    expect(logger.debug).toHaveBeenCalledWith("onFetchRequest", {
      method: "POST",
      url: "https://rpc.local",
      body: requestBody,
    });

    const failingRequest = {
      clone: jest.fn().mockReturnValue({
        text: jest.fn().mockRejectedValue(new Error("request-read-fail")),
      }),
    };

    await transportConfig.onFetchRequest(failingRequest);

    expect(logger.warn).toHaveBeenCalledWith("Failed to read request body", {
      err: expect.any(Error),
    });

    const responseBody = JSON.stringify({ ok: true });
    const responseClone = { text: jest.fn().mockResolvedValue(responseBody) };
    const response = {
      status: 200,
      statusText: "OK",
      clone: jest.fn().mockReturnValue(responseClone),
    };

    await transportConfig.onFetchResponse(response);

    expect(logger.debug).toHaveBeenCalledWith("onFetchResponse", {
      status: 200,
      statusText: "OK",
      body: responseBody,
    });

    const responseError = {
      clone: jest.fn().mockReturnValue({
        text: jest.fn().mockRejectedValue(new Error("read-fail")),
      }),
    };

    await transportConfig.onFetchResponse(responseError);

    expect(logger.warn).toHaveBeenCalledWith("Failed to read response body", {
      err: expect.any(Error),
    });
  });

  it("throws if sendTransactionsMaxRetries is less than 1", () => {
    expect(() => new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 0, 1000n, 1_000)).toThrow(
      "sendTransactionsMaxRetries must be at least 1",
    );
  });

  it("exposes the underlying public client", () => {
    expect(adapter.getBlockchainClient()).toBe(publicClientMock);
  });

  it("delegates to public client for getChainId, getBalance, estimateGasFees", async () => {
    publicClientMock.getChainId.mockResolvedValue(5);
    publicClientMock.getBalance.mockResolvedValue(123n);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 40n,
      maxPriorityFeePerGas: 3n,
    });

    await expect(adapter.getChainId()).resolves.toBe(5);
    await expect(adapter.getBalance("0xabc" as Address)).resolves.toBe(123n);
    await expect(adapter.estimateGasFees()).resolves.toEqual({
      maxFeePerGas: 40n,
      maxPriorityFeePerGas: 3n,
    });

    expect(publicClientMock.getChainId).toHaveBeenCalledTimes(1);
    expect(publicClientMock.getBalance).toHaveBeenCalledWith({ address: "0xabc" });
    expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(1);
  });

  it("uses default constructor parameters and default tx value", async () => {
    const defaultsAdapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient);

    // Use a retryable RPC error (InternalRpcError -32603) instead of TimeoutError
    const retryableError = Object.assign(new BaseError("Internal RPC error"), { code: -32603 });

    publicClientMock.getTransactionCount.mockResolvedValue(4);
    publicClientMock.estimateGas.mockResolvedValue(200n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 3n })
      .mockResolvedValueOnce({ maxFeePerGas: 8n, maxPriorityFeePerGas: 2n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw retryableError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await defaultsAdapter.sendSignedTransaction(contractAddress, calldata);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=3",
      { error: retryableError },
    );
    expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenNthCalledWith(1, {
      to: contractAddress,
      type: "eip1559",
      data: calldata,
      chainId: chain.id,
      gas: 200n,
      maxFeePerGas: 10n,
      maxPriorityFeePerGas: 3n,
      nonce: 4,
      value: 0n,
    });
    expect(contractSignerClient.sign).toHaveBeenNthCalledWith(2, {
      to: contractAddress,
      type: "eip1559",
      data: calldata,
      chainId: chain.id,
      gas: 220n,
      maxFeePerGas: 11n,
      maxPriorityFeePerGas: 3n,
      nonce: 4,
      value: 0n,
    });
  });

  it("successfully sends a signed transaction on the first attempt", async () => {
    publicClientMock.getTransactionCount.mockResolvedValue(7);
    publicClientMock.estimateGas.mockResolvedValue(21_000n);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 100n,
      maxPriorityFeePerGas: 2n,
    });
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 10n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledWith({
      to: contractAddress,
      type: "eip1559",
      data: calldata,
      chainId: chain.id,
      gas: 21_000n,
      maxFeePerGas: 100n,
      maxPriorityFeePerGas: 2n,
      nonce: 7,
      value: 10n,
    });
    expect(mockedParseSignature).toHaveBeenCalledWith("0xSIGNATURE");
    expect(mockedSerializeTransaction).toHaveBeenCalledWith(
      {
        to: contractAddress,
        type: "eip1559",
        data: calldata,
        chainId: chain.id,
        gas: 21_000n,
        maxFeePerGas: 100n,
        maxPriorityFeePerGas: 2n,
        nonce: 7,
        value: 10n,
      },
      { r: "0x1", s: "0x2", yParity: 1 },
    );
    expect(mockedSendRawTransaction).toHaveBeenCalledWith(publicClientMock, {
      serializedTransaction: "0x02serialized",
    });
    expect(mockedWaitForTransactionReceipt).toHaveBeenCalledWith(publicClientMock, { hash: "0xHASH" });
  });

  it("retries on retryable errors and applies gas bump multipliers", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 3, 1_000n, 300_000);

    // Use a retryable RPC error (ResourceUnavailableRpcError -32002) instead of TimeoutError
    const retryableError = Object.assign(new BaseError("Resource unavailable"), { code: -32002 });

    publicClientMock.getTransactionCount.mockResolvedValue(5);
    publicClientMock.estimateGas.mockResolvedValue(200n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 3n })
      .mockResolvedValueOnce({ maxFeePerGas: 8n, maxPriorityFeePerGas: 2n });
    contractSignerClient.getAddress.mockReturnValue("0xSIGNER");
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw retryableError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=3",
      { error: retryableError },
    );
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenNthCalledWith(1, {
      to: contractAddress,
      type: "eip1559",
      data: calldata,
      chainId: chain.id,
      gas: 200n,
      maxFeePerGas: 10n,
      maxPriorityFeePerGas: 3n,
      nonce: 5,
      value: 0n,
    });
    expect(contractSignerClient.sign).toHaveBeenNthCalledWith(2, {
      to: contractAddress,
      type: "eip1559",
      data: calldata,
      chainId: chain.id,
      gas: 220n,
      maxFeePerGas: 11n,
      maxPriorityFeePerGas: 3n,
      nonce: 5,
      value: 0n,
    });
  });

  it("does not retry when TimeoutError is thrown", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const timeoutError = new TimeoutError({
      body: { message: "timeout" },
      url: "local:test",
    });

    publicClientMock.getTransactionCount.mockResolvedValue(9);
    publicClientMock.estimateGas.mockResolvedValue(150n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 12n,
      maxPriorityFeePerGas: 4n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw timeoutError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(timeoutError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
    expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(1);
  });

  // TODO: Add working test for non-BaseError errors
  it.skip("retries non-BaseError errors", async () => {});

  it("rethrows ContractFunctionRevertedError without retrying", async () => {
    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(50n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 5n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    const revertError = new ContractFunctionRevertedError({
      abi: [] as any,
      functionName: "test",
      message: "execution reverted",
    });
    (revertError as any).data = { errorName: "RevertReason" };
    // Add code -32015 (VM execution error) to make it non-retryable
    Object.assign(revertError, { code: -32015 });

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw revertError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toThrow();

    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
  });

  it("rethrows ContractFunctionRevertedError without retrying when error data is missing", async () => {
    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(50n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 5n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    const revertError = new ContractFunctionRevertedError({
      abi: [] as any,
      functionName: "test",
      message: "execution reverted",
    });
    (revertError as any).data = undefined;
    // Add code -32015 (VM execution error) to make it non-retryable
    Object.assign(revertError, { code: -32015 });

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw revertError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toThrow();
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
  });

  it("throws after exhausting retryable error retries", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 1_000);

    // Use a retryable RPC error (LimitExceededRpcError -32005)
    const retryableError = Object.assign(new BaseError("Limit exceeded"), { code: -32005 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementation(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw retryableError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toThrow();

    expect(logger.warn).toHaveBeenCalledTimes(1);
    expect(logger.error).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempts exhausted sendTransactionsMaxRetries=2",
      { error: retryableError },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("retries on retryable HTTP status codes", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    // Use a retryable HTTP status code (500 Internal Server Error)
    const httpError = Object.assign(new BaseError("HTTP error"), { status: 500 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw httpError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error: httpError },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("does not retry on non-retryable HTTP status codes", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    // Use a non-retryable HTTP status code (400 Bad Request)
    const httpError = Object.assign(new BaseError("HTTP error"), { status: 400 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw httpError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(httpError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
    expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(1);
  });

  it("retries on WebSocketRequestError", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    // Use WebSocketRequestError name
    const wsError = Object.assign(new BaseError("WebSocket error"), { name: "WebSocketRequestError" });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw wsError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error: wsError },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("retries on UnknownRpcError", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    // Use UnknownRpcError name
    const unknownRpcError = Object.assign(new BaseError("Unknown RPC error"), { name: "UnknownRpcError" });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw unknownRpcError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error: unknownRpcError },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("retries on default case (error with no code/status/name)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    // Use a BaseError without code, status, or matching name properties
    const defaultError = new BaseError("Unknown error");

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw defaultError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error: defaultError },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("retries on unknown RPC error code (default case - line 346)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    // Use an RPC error code that's NOT in the explicit retry or non-retry lists
    // This should hit line 346 (default retry behavior)
    const unknownRpcError = Object.assign(new BaseError("Unknown RPC error"), { code: 9999 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw unknownRpcError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(publicClientMock.estimateFeesPerGas).toHaveBeenCalledTimes(2);
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error: unknownRpcError },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("does not retry on capability error code 5700 (lower boundary)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const capabilityError = Object.assign(new BaseError("Capability error"), { code: 5700 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw capabilityError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(capabilityError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on capability error code 5760 (upper boundary)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const capabilityError = Object.assign(new BaseError("Capability error"), { code: 5760 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw capabilityError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(capabilityError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("retries on RPC error code 5699 (just below capability range)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const error = Object.assign(new BaseError("RPC error"), { code: 5699 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw error;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("retries on RPC error code 5761 (just above capability range)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const error = Object.assign(new BaseError("RPC error"), { code: 5761 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw error;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("retries on HTTP status code 408 (Request Timeout)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const httpError = Object.assign(new BaseError("HTTP error"), { status: 408 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw httpError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error: httpError },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("retries on HTTP status code 429 (Too Many Requests)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const httpError = Object.assign(new BaseError("HTTP error"), { status: 429 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw httpError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error: httpError },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("retries on HTTP status code 502 (Bad Gateway)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const httpError = Object.assign(new BaseError("HTTP error"), { status: 502 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw httpError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error: httpError },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("retries on HTTP status code 503 (Service Unavailable)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const httpError = Object.assign(new BaseError("HTTP error"), { status: 503 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw httpError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error: httpError },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("retries on HTTP status code 504 (Gateway Timeout)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const httpError = Object.assign(new BaseError("HTTP error"), { status: 504 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas
      .mockResolvedValueOnce({ maxFeePerGas: 9n, maxPriorityFeePerGas: 1n })
      .mockResolvedValueOnce({ maxFeePerGas: 10n, maxPriorityFeePerGas: 1n });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout
      .mockImplementationOnce(async (fn: any, _opts?: any) => {
        await fn({ signal: null });
        throw httpError;
      })
      .mockImplementationOnce(async (fn: any, _opts?: any) => fn({ signal: null }));

    const receipt = await adapter.sendSignedTransaction(contractAddress, calldata, 0n);

    expect(receipt).toEqual({ transactionHash: "0xHASH", status: "success" });
    expect(logger.warn).toHaveBeenCalledWith(
      "sendSignedTransaction retry attempt failed attempt=1 sendTransactionsMaxRetries=2",
      { error: httpError },
    );
    expect(mockedWithTimeout).toHaveBeenCalledTimes(2);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(2);
  });

  it("does not retry on ParseRpcError (-32700)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const parseError = Object.assign(new BaseError("Parse error"), { code: -32700 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw parseError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(parseError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on InvalidRequestRpcError (-32600)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const invalidRequestError = Object.assign(new BaseError("Invalid request"), { code: -32600 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw invalidRequestError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(invalidRequestError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on MethodNotFoundRpcError (-32601)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const methodNotFoundError = Object.assign(new BaseError("Method not found"), { code: -32601 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw methodNotFoundError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(methodNotFoundError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on InvalidParamsRpcError (-32602)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const invalidParamsError = Object.assign(new BaseError("Invalid params"), { code: -32602 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw invalidParamsError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(invalidParamsError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on InvalidInputRpcError (-32000)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const invalidInputError = Object.assign(new BaseError("Invalid input"), { code: -32000 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw invalidInputError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(invalidInputError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on ResourceNotFoundRpcError (-32001)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const resourceNotFoundError = Object.assign(new BaseError("Resource not found"), { code: -32001 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw resourceNotFoundError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(resourceNotFoundError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on TransactionRejectedRpcError (-32003)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const transactionRejectedError = Object.assign(new BaseError("Transaction rejected"), { code: -32003 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw transactionRejectedError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(transactionRejectedError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on MethodNotSupportedRpcError (-32004)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const methodNotSupportedError = Object.assign(new BaseError("Method not supported"), { code: -32004 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw methodNotSupportedError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(methodNotSupportedError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on JsonRpcVersionUnsupportedError (-32006)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const jsonRpcVersionError = Object.assign(new BaseError("JSON-RPC version unsupported"), { code: -32006 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw jsonRpcVersionError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(jsonRpcVersionError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on UserRejectedRequestError (4001)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const userRejectedError = Object.assign(new BaseError("User rejected request"), { code: 4001 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw userRejectedError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(userRejectedError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on UserRejectedRequestError CAIP-25 (5000)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const userRejectedError = Object.assign(new BaseError("User rejected request"), { code: 5000 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw userRejectedError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(userRejectedError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on UnauthorizedProviderError (4100)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const unauthorizedError = Object.assign(new BaseError("Unauthorized provider"), { code: 4100 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw unauthorizedError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(unauthorizedError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on UnsupportedProviderMethodError (4200)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const unsupportedMethodError = Object.assign(new BaseError("Unsupported provider method"), { code: 4200 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw unsupportedMethodError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(unsupportedMethodError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });

  it("does not retry on SwitchChainError (4902)", async () => {
    adapter = new ViemBlockchainClientAdapter(logger, rpcUrl, chain, contractSignerClient, 2, 1_000n, 300_000);

    const switchChainError = Object.assign(new BaseError("Switch chain error"), { code: 4902 });

    publicClientMock.getTransactionCount.mockResolvedValue(1);
    publicClientMock.estimateGas.mockResolvedValue(100n);
    publicClientMock.getChainId.mockResolvedValue(chain.id);
    publicClientMock.estimateFeesPerGas.mockResolvedValue({
      maxFeePerGas: 9n,
      maxPriorityFeePerGas: 1n,
    });
    contractSignerClient.sign.mockResolvedValue("0xSIGNATURE");

    mockedWithTimeout.mockReset();
    mockedWithTimeout.mockImplementationOnce(async (fn: any, _opts?: any) => {
      await fn({ signal: null });
      throw switchChainError;
    });

    await expect(adapter.sendSignedTransaction(contractAddress, calldata, 0n)).rejects.toBe(switchChainError);
    expect(logger.error).toHaveBeenCalledWith("sendSignedTransaction failed and will not be retried", {
      decodedError: expect.any(Error),
    });
    expect(mockedWithTimeout).toHaveBeenCalledTimes(1);
    expect(contractSignerClient.sign).toHaveBeenCalledTimes(1);
  });
});
