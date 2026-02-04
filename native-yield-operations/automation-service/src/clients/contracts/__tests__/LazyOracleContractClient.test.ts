import type { ILogger, IBlockchainClient } from "@consensys/linea-shared-utils";
import type { Address, Hex, PublicClient, TransactionReceipt } from "viem";
import { InvalidInputRpcError } from "viem";
import { jest, describe, it, expect, beforeEach, afterEach, beforeAll } from "@jest/globals";

import { createLoggerMock } from "../../../__tests__/helpers/index.js";
import { LazyOracleABI } from "../../../core/abis/LazyOracle.js";
import { LazyOracleErrorsABI } from "../../../core/abis/errors/LazyOracleErrors.js";
import { OperationTrigger } from "../../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";

const LazyOracleCombinedABI = [...LazyOracleABI, ...LazyOracleErrorsABI] as const;

jest.mock("viem", () => {
  const actual = jest.requireActual<typeof import("viem")>("viem");
  return {
    ...actual,
    getContract: jest.fn(),
    encodeFunctionData: jest.fn(),
  };
});

import { encodeFunctionData, getContract } from "viem";

const mockedGetContract = getContract as jest.MockedFunction<typeof getContract>;
const mockedEncodeFunctionData = encodeFunctionData as jest.MockedFunction<typeof encodeFunctionData>;

let LazyOracleContractClient: typeof import("../LazyOracleContractClient.js").LazyOracleContractClient;

beforeAll(async () => {
  ({ LazyOracleContractClient } = await import("../LazyOracleContractClient.js"));
});

describe("LazyOracleContractClient", () => {
  // Semantic constants
  const CONTRACT_ADDRESS = "0x1111111111111111111111111111111111111111" as Address;
  const VAULT_ADDRESS = "0x2222222222222222222222222222222222222222" as Address;
  const POLL_INTERVAL_MS = 5_000;
  const EVENT_WATCH_TIMEOUT_MS = 30_000;
  const SAMPLE_BALANCE = 1_000_000_000_000_000_000n; // 1 ETH
  const SAMPLE_TX_HASH = "0xhash" as Hex;
  const SAMPLE_TREE_ROOT = "0xabc" as Hex;
  const SAMPLE_REPORT_CID = "cid";

  let logger: ILogger;
  let blockchainClient: jest.Mocked<IBlockchainClient<PublicClient, TransactionReceipt>>;
  let watchContractEvent: jest.Mock;
  let stopWatching: jest.Mock;
  let publicClient: PublicClient;
  let contractStub: any;

  // Factory functions
  const createMockBlockchainClient = (): jest.Mocked<IBlockchainClient<PublicClient, TransactionReceipt>> => ({
    getBlockchainClient: jest.fn(),
    getBalance: jest.fn(),
    sendSignedTransaction: jest.fn(),
  } as any);

  const createMockContract = () => ({
    abi: LazyOracleCombinedABI,
    read: {
      latestReportData: jest.fn(),
    },
    simulate: {
      updateVaultData: jest.fn(),
    },
  });

  const createLatestReportDataResponse = (
    timestamp: bigint = 123n,
    refSlot: bigint = 456n,
    treeRoot: Hex = SAMPLE_TREE_ROOT,
    reportCid: string = SAMPLE_REPORT_CID,
  ) => [timestamp, refSlot, treeRoot, reportCid] as const;

  const createUpdateVaultDataParams = (overrides?: Partial<Parameters<typeof LazyOracleContractClient.prototype.updateVaultData>[0]>) => ({
    vault: VAULT_ADDRESS,
    totalValue: 1n,
    cumulativeLidoFees: 2n,
    liabilityShares: 3n,
    maxLiabilityShares: 4n,
    slashingReserve: 5n,
    proof: ["0x01"] as Hex[],
    ...overrides,
  });

  const createVaultReportEvent = (
    timestamp: bigint = 123n,
    refSlot: bigint = 456n,
    root: Hex = SAMPLE_TREE_ROOT,
    cid: string = SAMPLE_REPORT_CID,
    txHash: Hex = SAMPLE_TX_HASH,
  ) => ({
    removed: false,
    args: { timestamp, refSlot, root, cid },
    transactionHash: txHash,
  });

  const createClient = () =>
    new LazyOracleContractClient(logger, blockchainClient, CONTRACT_ADDRESS, POLL_INTERVAL_MS, EVENT_WATCH_TIMEOUT_MS);

  beforeEach(() => {
    jest.clearAllMocks();
    jest.useFakeTimers();
    logger = createLoggerMock();
    blockchainClient = createMockBlockchainClient();
    stopWatching = jest.fn();
    watchContractEvent = jest.fn().mockReturnValue(stopWatching);
    publicClient = { watchContractEvent } as unknown as PublicClient;
    blockchainClient.getBlockchainClient.mockReturnValue(publicClient);
    contractStub = createMockContract();
    mockedGetContract.mockReturnValue(contractStub);
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("initializes viem contract with correct configuration", () => {
    // Arrange
    // (setup in beforeEach)

    // Act
    const client = createClient();

    // Assert
    expect(mockedGetContract).toHaveBeenCalledWith({
      abi: LazyOracleCombinedABI,
      address: CONTRACT_ADDRESS,
      client: publicClient,
    });
    expect(client.getAddress()).toBe(CONTRACT_ADDRESS);
    expect(client.getContract()).toBe(contractStub);
  });

  it("retrieves contract balance from blockchain client", async () => {
    // Arrange
    blockchainClient.getBalance.mockResolvedValueOnce(SAMPLE_BALANCE);
    const client = createClient();

    // Act
    const result = await client.getBalance();

    // Assert
    expect(result).toBe(SAMPLE_BALANCE);
    expect(blockchainClient.getBalance).toHaveBeenCalledWith(CONTRACT_ADDRESS);
  });

  it("transforms latest report data into normalized structure", async () => {
    // Arrange
    const client = createClient();
    const rawResponse = createLatestReportDataResponse();
    contractStub.read.latestReportData.mockResolvedValueOnce(rawResponse);

    // Act
    const report = await client.latestReportData();

    // Assert
    expect(report).toEqual({
      timestamp: 123n,
      refSlot: 456n,
      treeRoot: SAMPLE_TREE_ROOT,
      reportCid: SAMPLE_REPORT_CID,
    });
    expect(logger.debug).toHaveBeenCalledWith("latestReportData", {
      returnVal: report,
    });
  });

  it("encodes and sends updateVaultData transaction", async () => {
    // Arrange
    const client = createClient();
    const params = createUpdateVaultDataParams();
    const calldata = "0xdeadbeef" as Hex;
    const receipt = { transactionHash: SAMPLE_TX_HASH } as unknown as TransactionReceipt;

    mockedEncodeFunctionData.mockReturnValueOnce(calldata);
    blockchainClient.sendSignedTransaction.mockResolvedValueOnce(receipt);

    // Act
    const result = await client.updateVaultData(params);

    // Assert
    expect(result).toBe(receipt);
    expect(logger.debug).toHaveBeenCalledWith("updateVaultData started", { params });
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "updateVaultData",
      args: [
        params.vault,
        params.totalValue,
        params.cumulativeLidoFees,
        params.liabilityShares,
        params.maxLiabilityShares,
        params.slashingReserve,
        params.proof,
      ],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(
      CONTRACT_ADDRESS,
      calldata,
      undefined,
      LazyOracleCombinedABI,
    );
    expect(logger.info).toHaveBeenCalledWith("updateVaultData succeeded, txHash=0xhash", { params });
  });

  it("resolves with event result when VaultsReportDataUpdated event is received", async () => {
    // Arrange
    const client = createClient();
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");
    const eventLog = createVaultReportEvent();
    const promise = client.waitForVaultsReportDataUpdatedEvent();

    const watchArgs = watchContractEvent.mock.calls[0]?.[0] as any;

    // Act
    watchArgs?.onLogs?.([eventLog as any]);
    const result = await promise;

    // Assert
    expect(watchContractEvent).toHaveBeenCalledWith({
      address: CONTRACT_ADDRESS,
      abi: contractStub.abi,
      eventName: "VaultsReportDataUpdated",
      pollingInterval: POLL_INTERVAL_MS,
      onLogs: expect.any(Function),
      onError: expect.any(Function),
    });
    expect(result).toEqual({
      result: OperationTrigger.VAULTS_REPORT_DATA_UPDATED_EVENT,
      txHash: SAMPLE_TX_HASH,
      report: {
        timestamp: 123n,
        refSlot: 456n,
        treeRoot: SAMPLE_TREE_ROOT,
        reportCid: SAMPLE_REPORT_CID,
      },
    });
    expect(clearTimeoutSpy).toHaveBeenCalledTimes(1);
    expect(stopWatching).toHaveBeenCalledTimes(1);
    expect(logger.info).toHaveBeenCalledWith(
      "waitForVaultsReportDataUpdatedEvent detected",
      expect.objectContaining({
        result: expect.objectContaining({
          result: OperationTrigger.VAULTS_REPORT_DATA_UPDATED_EVENT,
        }),
      }),
    );
    clearTimeoutSpy.mockRestore();
  });

  it("resolves with timeout result when no event arrives before deadline", async () => {
    // Arrange
    const client = createClient();
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");
    const promise = client.waitForVaultsReportDataUpdatedEvent();

    // Act
    jest.advanceTimersByTime(EVENT_WATCH_TIMEOUT_MS);
    const result = await promise;

    // Assert
    expect(logger.info).toHaveBeenCalledWith(
      `waitForVaultsReportDataUpdatedEvent started with timeout=${EVENT_WATCH_TIMEOUT_MS}ms`,
    );
    expect(result).toEqual({ result: OperationTrigger.TIMEOUT });
    expect(clearTimeoutSpy).toHaveBeenCalled();
    expect(stopWatching).toHaveBeenCalledTimes(1);
    expect(logger.info).toHaveBeenCalledWith(
      `waitForVaultsReportDataUpdatedEvent timed out after timeout=${EVENT_WATCH_TIMEOUT_MS}ms`,
    );
    clearTimeoutSpy.mockRestore();
  });

  it("logs watcher errors and continues waiting for event", async () => {
    // Arrange
    const client = createClient();
    const promise = client.waitForVaultsReportDataUpdatedEvent();
    const watchArgs = watchContractEvent.mock.calls[0]?.[0] as any;
    const error = new Error("boom");

    // Act
    watchArgs?.onError?.(error);
    jest.advanceTimersByTime(EVENT_WATCH_TIMEOUT_MS);
    const result = await promise;

    // Assert
    expect(logger.error).toHaveBeenCalledWith("waitForVaultsReportDataUpdatedEvent error", { error });
    expect(result).toEqual({ result: OperationTrigger.TIMEOUT });
    expect(stopWatching).toHaveBeenCalledTimes(1);
  });

  it("logs warning for InvalidInputRpcError and continues waiting", async () => {
    // Arrange
    const client = createClient();
    const promise = client.waitForVaultsReportDataUpdatedEvent();
    const watchArgs = watchContractEvent.mock.calls[0]?.[0] as any;
    const invalidInputError = new InvalidInputRpcError(new Error("Filter expired"));

    // Act
    watchArgs?.onError?.(invalidInputError);
    jest.advanceTimersByTime(EVENT_WATCH_TIMEOUT_MS);
    const result = await promise;

    // Assert
    expect(logger.warn).toHaveBeenCalledWith(
      "waitForVaultsReportDataUpdatedEvent: Filter expired, will be recreated by Viem framework",
      { error: invalidInputError },
    );
    expect(logger.error).not.toHaveBeenCalledWith("waitForVaultsReportDataUpdatedEvent error", expect.anything());
    expect(result).toEqual({ result: OperationTrigger.TIMEOUT });
    expect(stopWatching).toHaveBeenCalledTimes(1);
  });

  it("warns when reorged logs arrive then resolves with valid event", async () => {
    // Arrange
    const client = createClient();
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");
    const promise = client.waitForVaultsReportDataUpdatedEvent();
    const watchArgs = watchContractEvent.mock.calls[0]?.[0] as any;
    const validEvent = createVaultReportEvent(123n, 456n, "0xbeef" as Hex);

    // Act
    watchArgs?.onLogs?.([{ removed: true }] as any);
    watchArgs?.onLogs?.([validEvent as any]);
    const result = await promise;

    // Assert
    expect(logger.warn).toHaveBeenCalledWith(
      "waitForVaultsReportDataUpdatedEvent: Dropped VaultsReportDataUpdated event",
    );
    expect(result).toEqual({
      result: OperationTrigger.VAULTS_REPORT_DATA_UPDATED_EVENT,
      txHash: SAMPLE_TX_HASH,
      report: {
        timestamp: 123n,
        refSlot: 456n,
        treeRoot: "0xbeef" as Hex,
        reportCid: SAMPLE_REPORT_CID,
      },
    });
    expect(clearTimeoutSpy).toHaveBeenCalledTimes(1);
    expect(stopWatching).toHaveBeenCalledTimes(1);
    clearTimeoutSpy.mockRestore();
  });

  it("logs debug details when filtering out reorged logs", async () => {
    // Arrange
    const client = createClient();
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");
    const promise = client.waitForVaultsReportDataUpdatedEvent();
    const watchArgs = watchContractEvent.mock.calls[0]?.[0] as any;
    const removedLog = { removed: true, args: {}, transactionHash: "0xdead" };
    const validLog = createVaultReportEvent(1n, 2n, "0xroot" as Hex, SAMPLE_REPORT_CID, "0xlive" as Hex);
    const logs = [removedLog, validLog];

    // Act
    watchArgs?.onLogs?.(logs as any);
    const result = await promise;

    // Assert
    expect(logger.debug).toHaveBeenCalledWith("waitForVaultsReportDataUpdatedEvent: Ignored removed reorg logs", {
      logs,
    });
    expect(result).toEqual({
      result: OperationTrigger.VAULTS_REPORT_DATA_UPDATED_EVENT,
      txHash: "0xlive" as Hex,
      report: {
        timestamp: 1n,
        refSlot: 2n,
        treeRoot: "0xroot" as Hex,
        reportCid: SAMPLE_REPORT_CID,
      },
    });
    expect(clearTimeoutSpy).toHaveBeenCalledTimes(1);
    expect(stopWatching).toHaveBeenCalledTimes(1);
    clearTimeoutSpy.mockRestore();
  });

  it("skips events with incomplete arguments and continues waiting", async () => {
    // Arrange
    const client = createClient();
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");
    const promise = client.waitForVaultsReportDataUpdatedEvent();
    const watchArgs = watchContractEvent.mock.calls[0]?.[0] as any;
    const incompleteLog = {
      removed: false,
      args: {
        timestamp: 123n,
        refSlot: undefined,
        root: "0xroot" as Hex,
        cid: SAMPLE_REPORT_CID,
      },
      transactionHash: "0xincomplete" as Hex,
    };

    // Act
    watchArgs?.onLogs?.([incompleteLog as any]);
    jest.advanceTimersByTime(EVENT_WATCH_TIMEOUT_MS);
    const result = await promise;

    // Assert
    expect(logger.debug).toHaveBeenCalledWith("waitForVaultsReportDataUpdatedEvent: Event args incomplete, skipping", {
      hasTimestamp: true,
      hasRefSlot: false,
      hasRoot: true,
      hasCid: true,
    });
    expect(logger.info).not.toHaveBeenCalledWith("waitForVaultsReportDataUpdatedEvent detected", expect.anything());
    expect(result).toEqual({ result: OperationTrigger.TIMEOUT });
    expect(stopWatching).toHaveBeenCalledTimes(1);
    clearTimeoutSpy.mockRestore();
  });
});
