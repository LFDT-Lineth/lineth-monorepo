import { mock, MockProxy } from "jest-mock-extended";
import type { ILogger, IBlockchainClient } from "@consensys/linea-shared-utils";
import type { Address, Hex, PublicClient, TransactionReceipt } from "viem";
import { InvalidInputRpcError } from "viem";
import { LazyOracleABI } from "../../../core/abis/LazyOracle.js";
import { OperationTrigger } from "../../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
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
  const contractAddress = "0x1111111111111111111111111111111111111111" as Address;
  const pollIntervalMs = 5_000;
  const eventWatchTimeoutMs = 30_000;

  let logger: MockProxy<ILogger>;
  let blockchainClient: MockProxy<IBlockchainClient<PublicClient, TransactionReceipt>>;
  let watchContractEvent: jest.Mock;
  let stopWatching: jest.Mock;
  let publicClient: PublicClient;
  let contractStub: any;

  const createClient = () =>
    new LazyOracleContractClient(logger, blockchainClient, contractAddress, pollIntervalMs, eventWatchTimeoutMs);

  beforeEach(() => {
    jest.clearAllMocks();
    jest.useFakeTimers();
    logger = mock<ILogger>();
    blockchainClient = mock<IBlockchainClient<PublicClient, TransactionReceipt>>();
    stopWatching = jest.fn();
    watchContractEvent = jest.fn().mockReturnValue(stopWatching);
    publicClient = { watchContractEvent } as unknown as PublicClient;
    blockchainClient.getBlockchainClient.mockReturnValue(publicClient);
    contractStub = {
      abi: LazyOracleABI,
      read: {
        latestReportData: jest.fn(),
      },
      simulate: {
        updateVaultData: jest.fn(),
      },
    };
    mockedGetContract.mockReturnValue(contractStub);
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("initializes the viem contract and exposes getters", () => {
    const client = createClient();

    expect(mockedGetContract).toHaveBeenCalledWith({
      abi: LazyOracleABI,
      address: contractAddress,
      client: publicClient,
    });
    expect(client.getAddress()).toBe(contractAddress);
    expect(client.getContract()).toBe(contractStub);
  });

  it("returns latest report data with normalized structure", async () => {
    const client = createClient();
    const latest = [123n, 456n, "0xabc" as Hex, "cid"] as const;
    contractStub.read.latestReportData.mockResolvedValueOnce(latest);

    const report = await client.latestReportData();

    expect(report).toEqual({
      timestamp: latest[0],
      refSlot: latest[1],
      treeRoot: latest[2],
      reportCid: latest[3],
    });
    expect(logger.debug).toHaveBeenCalledWith("latestReportData", {
      returnVal: report,
    });
  });

  it("encodes calldata and relays updateVaultData to the blockchain client", async () => {
    const client = createClient();
    const calldata = "0xdeadbeef" as Hex;
    const receipt = { transactionHash: "0xhash" } as unknown as TransactionReceipt;
    const params = {
      vault: "0x2222222222222222222222222222222222222222" as Address,
      totalValue: 1n,
      cumulativeLidoFees: 2n,
      liabilityShares: 3n,
      maxLiabilityShares: 4n,
      slashingReserve: 5n,
      proof: ["0x01"] as Hex[],
    };

    mockedEncodeFunctionData.mockReturnValueOnce(calldata);
    blockchainClient.sendSignedTransaction.mockResolvedValueOnce(receipt);

    const result = await client.updateVaultData(params);

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
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(contractAddress, calldata);
    expect(logger.info).toHaveBeenCalledWith("updateVaultData succeeded, txHash=0xhash", { params });
  });

  it("resolves with VaultReportResult when VaultsReportDataUpdated event arrives", async () => {
    const client = createClient();
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");
    const promise = client.waitForVaultsReportDataUpdatedEvent();

    expect(watchContractEvent).toHaveBeenCalledWith({
      address: contractAddress,
      abi: contractStub.abi,
      eventName: "VaultsReportDataUpdated",
      pollingInterval: pollIntervalMs,
      onLogs: expect.any(Function),
      onError: expect.any(Function),
    });

    const watchArgs = watchContractEvent.mock.calls[0][0];
    expect(stopWatching).not.toHaveBeenCalled();
    const log = {
      removed: false,
      args: {
        timestamp: 123n,
        refSlot: 456n,
        root: "0xabc" as Hex,
        cid: "cid",
      },
      transactionHash: "0xhash" as Hex,
    };

    watchArgs.onLogs?.([log as any]);
    await expect(promise).resolves.toEqual({
      result: OperationTrigger.VAULTS_REPORT_DATA_UPDATED_EVENT,
      txHash: log.transactionHash,
      report: {
        timestamp: log.args.timestamp,
        refSlot: log.args.refSlot,
        treeRoot: log.args.root,
        reportCid: log.args.cid,
      },
    });
    expect(clearTimeoutSpy).toHaveBeenCalledTimes(1);
    expect(stopWatching).toHaveBeenCalledTimes(1);
    clearTimeoutSpy.mockRestore();
    expect(logger.info).toHaveBeenCalledWith(
      "waitForVaultsReportDataUpdatedEvent detected",
      expect.objectContaining({
        result: expect.objectContaining({
          result: OperationTrigger.VAULTS_REPORT_DATA_UPDATED_EVENT,
        }),
      }),
    );
  });

  it("resolves with timeout result when no event is observed", async () => {
    const client = createClient();
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");
    const promise = client.waitForVaultsReportDataUpdatedEvent();

    expect(logger.info).toHaveBeenCalledWith(
      `waitForVaultsReportDataUpdatedEvent started with timeout=${eventWatchTimeoutMs}ms`,
    );

    expect(stopWatching).not.toHaveBeenCalled();
    jest.advanceTimersByTime(eventWatchTimeoutMs);
    await expect(promise).resolves.toEqual({ result: OperationTrigger.TIMEOUT });
    expect(clearTimeoutSpy).toHaveBeenCalled();
    expect(stopWatching).toHaveBeenCalledTimes(1);
    clearTimeoutSpy.mockRestore();
    expect(logger.info).toHaveBeenCalledWith(
      `waitForVaultsReportDataUpdatedEvent timed out after timeout=${eventWatchTimeoutMs}ms`,
    );
  });

  it("logs errors emitted by the watcher and continues waiting", async () => {
    const client = createClient();
    const promise = client.waitForVaultsReportDataUpdatedEvent();
    const watchArgs = watchContractEvent.mock.calls[0][0];
    const failure = new Error("boom");

    watchArgs.onError?.(failure);

    expect(logger.error).toHaveBeenCalledWith("waitForVaultsReportDataUpdatedEvent error", { error: failure });

    jest.advanceTimersByTime(eventWatchTimeoutMs);
    await expect(promise).resolves.toEqual({ result: OperationTrigger.TIMEOUT });
    expect(stopWatching).toHaveBeenCalledTimes(1);
  });

  it("warns and continues waiting when InvalidInputRpcError is emitted (filter expired)", async () => {
    const client = createClient();
    const promise = client.waitForVaultsReportDataUpdatedEvent();
    const watchArgs = watchContractEvent.mock.calls[0][0];
    const invalidInputError = new InvalidInputRpcError(new Error("Filter expired"));

    watchArgs.onError?.(invalidInputError);

    expect(logger.warn).toHaveBeenCalledWith(
      "waitForVaultsReportDataUpdatedEvent: Filter expired, will be recreated by Viem framework",
      { error: invalidInputError },
    );
    expect(logger.error).not.toHaveBeenCalledWith("waitForVaultsReportDataUpdatedEvent error", expect.anything());

    jest.advanceTimersByTime(eventWatchTimeoutMs);
    await expect(promise).resolves.toEqual({ result: OperationTrigger.TIMEOUT });
    expect(stopWatching).toHaveBeenCalledTimes(1);
  });

  it("warns when all received logs are removed before resolving later events", async () => {
    const client = createClient();
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");
    const promise = client.waitForVaultsReportDataUpdatedEvent();
    const watchArgs = watchContractEvent.mock.calls[0][0];

    watchArgs.onLogs?.([{ removed: true }] as any);

    expect(logger.warn).toHaveBeenCalledWith(
      "waitForVaultsReportDataUpdatedEvent: Dropped VaultsReportDataUpdated event",
    );
    expect(stopWatching).not.toHaveBeenCalled();

    const log = {
      removed: false,
      args: {
        timestamp: 123n,
        refSlot: 456n,
        root: "0xbeef" as Hex,
        cid: "cid",
      },
      transactionHash: "0xhash" as Hex,
    };

    watchArgs.onLogs?.([log as any]);

    await expect(promise).resolves.toEqual({
      result: OperationTrigger.VAULTS_REPORT_DATA_UPDATED_EVENT,
      txHash: log.transactionHash,
      report: {
        timestamp: log.args.timestamp,
        refSlot: log.args.refSlot,
        treeRoot: log.args.root,
        reportCid: log.args.cid,
      },
    });
    expect(clearTimeoutSpy).toHaveBeenCalledTimes(1);
    expect(stopWatching).toHaveBeenCalledTimes(1);
    clearTimeoutSpy.mockRestore();
  });

  it("logs debug details when reorg logs are filtered out", async () => {
    const client = createClient();
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");
    const promise = client.waitForVaultsReportDataUpdatedEvent();
    const watchArgs = watchContractEvent.mock.calls[0][0];

    const logs = [
      { removed: true, args: {}, transactionHash: "0xdead" },
      {
        removed: false,
        args: {
          timestamp: 1n,
          refSlot: 2n,
          root: "0xroot" as Hex,
          cid: "cid",
        },
        transactionHash: "0xlive" as Hex,
      },
    ];

    watchArgs.onLogs?.(logs as any);

    expect(logger.debug).toHaveBeenCalledWith("waitForVaultsReportDataUpdatedEvent: Ignored removed reorg logs", {
      logs,
    });

    await expect(promise).resolves.toEqual({
      result: OperationTrigger.VAULTS_REPORT_DATA_UPDATED_EVENT,
      txHash: logs[1].transactionHash as Hex,
      report: {
        timestamp: logs[1].args.timestamp,
        refSlot: logs[1].args.refSlot,
        treeRoot: logs[1].args.root,
        reportCid: logs[1].args.cid,
      },
    });
    expect(clearTimeoutSpy).toHaveBeenCalledTimes(1);
    expect(stopWatching).toHaveBeenCalledTimes(1);
    clearTimeoutSpy.mockRestore();
  });

  it("ignores logs that lack the expected arguments", async () => {
    const client = createClient();
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");
    const promise = client.waitForVaultsReportDataUpdatedEvent();
    const watchArgs = watchContractEvent.mock.calls[0][0];

    const incompleteLog = {
      removed: false,
      args: {
        timestamp: 123n,
        refSlot: undefined,
        root: "0xroot" as Hex,
        cid: "cid",
      },
      transactionHash: "0xincomplete" as Hex,
    };

    watchArgs.onLogs?.([incompleteLog as any]);

    expect(stopWatching).not.toHaveBeenCalled();
    expect(logger.info).not.toHaveBeenCalledWith("waitForVaultsReportDataUpdatedEvent detected", expect.anything());
    expect(clearTimeoutSpy).not.toHaveBeenCalled();

    jest.advanceTimersByTime(eventWatchTimeoutMs);
    await expect(promise).resolves.toEqual({ result: OperationTrigger.TIMEOUT });
    expect(clearTimeoutSpy).toHaveBeenCalled();
    expect(stopWatching).toHaveBeenCalledTimes(1);
    clearTimeoutSpy.mockRestore();
  });
});
