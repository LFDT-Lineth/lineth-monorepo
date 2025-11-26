import { mock, MockProxy } from "jest-mock-extended";
import type { ILogger, IBlockchainClient } from "@consensys/linea-shared-utils";
import type { Address, Hex, PublicClient, TransactionReceipt } from "viem";
import { YieldManagerABI } from "../../../core/abis/YieldManager.js";
import { StakingVaultABI } from "../../../core/abis/StakingVault.js";
import { RebalanceDirection } from "../../../core/entities/RebalanceRequirement.js";
import type { WithdrawalRequests } from "../../../core/entities/LidoStakingVaultWithdrawalParams.js";
import { ONE_ETHER } from "@consensys/linea-shared-utils";

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return {
    ...actual,
    getContract: jest.fn(),
    encodeFunctionData: jest.fn(),
    parseEventLogs: jest.fn(),
    encodeAbiParameters: jest.fn(),
  };
});

import { getContract, encodeFunctionData, parseEventLogs, encodeAbiParameters } from "viem";

const mockedGetContract = getContract as jest.MockedFunction<typeof getContract>;
const mockedEncodeFunctionData = encodeFunctionData as jest.MockedFunction<typeof encodeFunctionData>;
const mockedParseEventLogs = parseEventLogs as jest.MockedFunction<typeof parseEventLogs>;
const mockedEncodeAbiParameters = encodeAbiParameters as jest.MockedFunction<typeof encodeAbiParameters>;

let YieldManagerContractClient: typeof import("../YieldManagerContractClient.js").YieldManagerContractClient;

beforeAll(async () => {
  ({ YieldManagerContractClient } = await import("../YieldManagerContractClient.js"));
});

describe("YieldManagerContractClient", () => {
  const contractAddress = "0xcccccccccccccccccccccccccccccccccccccccc" as Address;
  const yieldProvider = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" as Address;
  const l2Recipient = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" as Address;
  const l1MessageServiceAddress = "0x9999999999999999999999999999999999999999" as Address;
  const stakingVaultAddress = "0x8888888888888888888888888888888888888888" as Address;

  let logger: MockProxy<ILogger>;
  let blockchainClient: MockProxy<IBlockchainClient<PublicClient, TransactionReceipt>>;
  let publicClient: { readContract: jest.Mock } & Record<string, unknown>;
  let contractStub: {
    abi: typeof YieldManagerABI;
    read: Record<string, jest.Mock>;
    simulate: Record<string, jest.Mock>;
  };

  const defaultYieldProviderData = {
    yieldProviderVendor: 0,
    isStakingPaused: false,
    isOssificationInitiated: false,
    isOssified: false,
    primaryEntrypoint: l2Recipient,
    ossifiedEntrypoint: stakingVaultAddress,
    yieldProviderIndex: 0n,
    userFunds: 0n,
    yieldReportedCumulative: 0n,
    lstLiabilityPrincipal: 0n,
  };

  const buildReceipt = (logs: Array<{ address: string; data: string; topics: string[] }>): TransactionReceipt =>
    ({
      logs,
    }) as unknown as TransactionReceipt;

  const createClient = ({
    rebalanceToleranceBps = 100,
    minWithdrawalThresholdEth = 0n,
  }: {
    rebalanceToleranceBps?: number;
    minWithdrawalThresholdEth?: bigint;
  } = {}) =>
    new YieldManagerContractClient(
      logger,
      blockchainClient,
      contractAddress,
      rebalanceToleranceBps,
      minWithdrawalThresholdEth,
    );

  beforeEach(() => {
    jest.clearAllMocks();
    logger = mock<ILogger>();
    blockchainClient = mock<IBlockchainClient<PublicClient, TransactionReceipt>>();
    publicClient = {
      readContract: jest.fn(),
    };

    blockchainClient.getBlockchainClient.mockReturnValue(publicClient as unknown as PublicClient);

    contractStub = {
      abi: YieldManagerABI,
      read: {
        L1_MESSAGE_SERVICE: jest.fn(),
        getTotalSystemBalance: jest.fn(),
        getEffectiveTargetWithdrawalReserve: jest.fn(),
        getTargetReserveDeficit: jest.fn(),
        isStakingPaused: jest.fn(),
        isOssificationInitiated: jest.fn(),
        isOssified: jest.fn(),
        getYieldProviderData: jest.fn().mockResolvedValue(defaultYieldProviderData),
      },
      simulate: {
        withdrawableValue: jest.fn(),
        reportYield: jest.fn(),
      },
    };

    mockedGetContract.mockReturnValue(contractStub as any);
    blockchainClient.getBalance.mockResolvedValue(0n);
    blockchainClient.sendSignedTransaction.mockResolvedValue({
      transactionHash: "0xreceipt",
    } as unknown as TransactionReceipt);
    contractStub.read.L1_MESSAGE_SERVICE.mockResolvedValue(l1MessageServiceAddress);
    contractStub.read.getTotalSystemBalance.mockResolvedValue(0n);
    contractStub.read.getEffectiveTargetWithdrawalReserve.mockResolvedValue(0n);
    contractStub.read.getTargetReserveDeficit.mockResolvedValue(0n);
    contractStub.read.isStakingPaused.mockResolvedValue(false);
    contractStub.read.isOssificationInitiated.mockResolvedValue(false);
    contractStub.read.isOssified.mockResolvedValue(false);
    contractStub.simulate.withdrawableValue.mockResolvedValue({ result: 0n });
  });

  it("initializes the viem contract and exposes address & contract accessors", () => {
    const client = createClient();

    expect(mockedGetContract).toHaveBeenCalledWith({
      abi: YieldManagerABI,
      address: contractAddress,
      client: publicClient,
    });
    expect(client.getAddress()).toBe(contractAddress);
    expect(client.getContract()).toBe(contractStub);
  });

  it("delegates simple reads to the viem contract", async () => {
    const stakingPaused = true;
    const ossificationInitiated = true;
    const ossified = true;
    contractStub.read.getTotalSystemBalance.mockResolvedValueOnce(123n);
    contractStub.read.getEffectiveTargetWithdrawalReserve.mockResolvedValueOnce(456n);
    contractStub.read.getTargetReserveDeficit.mockResolvedValueOnce(789n);
    contractStub.read.isStakingPaused.mockResolvedValueOnce(stakingPaused);
    contractStub.read.isOssificationInitiated.mockResolvedValueOnce(ossificationInitiated);
    contractStub.read.isOssified.mockResolvedValueOnce(ossified);

    const client = createClient();

    await expect(client.getTotalSystemBalance()).resolves.toBe(123n);
    await expect(client.getEffectiveTargetWithdrawalReserve()).resolves.toBe(456n);
    await expect(client.getTargetReserveDeficit()).resolves.toBe(789n);
    await expect(client.L1_MESSAGE_SERVICE()).resolves.toBe(l1MessageServiceAddress);
    await expect(client.isStakingPaused(yieldProvider)).resolves.toBe(stakingPaused);
    await expect(client.isOssificationInitiated(yieldProvider)).resolves.toBe(ossificationInitiated);
    await expect(client.isOssified(yieldProvider)).resolves.toBe(ossified);

    expect(contractStub.read.getTotalSystemBalance).toHaveBeenCalledTimes(1);
    expect(contractStub.read.getEffectiveTargetWithdrawalReserve).toHaveBeenCalledTimes(1);
    expect(contractStub.read.getTargetReserveDeficit).toHaveBeenCalledTimes(1);
    expect(contractStub.read.L1_MESSAGE_SERVICE).toHaveBeenCalledTimes(1);
    expect(contractStub.read.isStakingPaused).toHaveBeenCalledWith([yieldProvider]);
    expect(contractStub.read.isOssificationInitiated).toHaveBeenCalledWith([yieldProvider]);
    expect(contractStub.read.isOssified).toHaveBeenCalledWith([yieldProvider]);
  });

  it("reads withdrawableValue via simulate and returns the result", async () => {
    const withdrawable = 42n;
    contractStub.simulate.withdrawableValue.mockResolvedValueOnce({ result: withdrawable });

    const client = createClient();
    const result = await client.withdrawableValue(yieldProvider);

    expect(contractStub.simulate.withdrawableValue).toHaveBeenCalledWith([yieldProvider]);
    expect(result).toBe(withdrawable);
  });

  it("peeks yield report via simulate and returns mapped YieldReport", async () => {
    const newReportedYield = 100n;
    const outstandingNegativeYield = 25n;
    contractStub.simulate.reportYield.mockResolvedValueOnce({
      result: [newReportedYield, outstandingNegativeYield],
    });

    const client = createClient();
    const result = await client.peekYieldReport(yieldProvider, l2Recipient);

    expect(contractStub.simulate.reportYield).toHaveBeenCalledWith([yieldProvider, l2Recipient]);
    expect(result).toEqual({
      yieldAmount: newReportedYield,
      outstandingNegativeYield,
      yieldProvider,
    });
  });

  it("returns undefined when peekYieldReport simulation fails", async () => {
    contractStub.simulate.reportYield.mockRejectedValueOnce(new Error("Simulation failed"));

    const client = createClient();
    const result = await client.peekYieldReport(yieldProvider, l2Recipient);

    expect(contractStub.simulate.reportYield).toHaveBeenCalledWith([yieldProvider, l2Recipient]);
    expect(result).toBeUndefined();
    expect(logger.debug).toHaveBeenCalledWith(
      `peekYieldReport failed, yieldProvider=${yieldProvider}, l2YieldRecipient=${l2Recipient}`,
      { error: expect.any(Error) },
    );
  });

  it("encodes calldata and sends fundYieldProvider transactions", async () => {
    const amount = 100n;
    const calldata = "0xcalldata" as Hex;
    const txReceipt = { transactionHash: "0xhash" } as unknown as TransactionReceipt;
    mockedEncodeFunctionData.mockReturnValueOnce(calldata);
    blockchainClient.sendSignedTransaction.mockResolvedValueOnce(txReceipt);

    const client = createClient();
    const receipt = await client.fundYieldProvider(yieldProvider, amount);

    expect(logger.debug).toHaveBeenCalledWith(
      `fundYieldProvider started, yieldProvider=${yieldProvider}, amount=${amount.toString()}`,
    );
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "fundYieldProvider",
      args: [yieldProvider, amount],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(contractAddress, calldata);
    expect(logger.info).toHaveBeenCalledWith(
      `fundYieldProvider succeeded, yieldProvider=${yieldProvider}, amount=${amount.toString()}, txHash=${txReceipt.transactionHash}`,
    );
    expect(receipt).toBe(txReceipt);
  });

  it("reports yield with encoded calldata", async () => {
    const calldata = "0xreport" as Hex;
    const txReceipt = { transactionHash: "0xreporthash" } as unknown as TransactionReceipt;
    mockedEncodeFunctionData.mockReturnValueOnce(calldata);
    blockchainClient.sendSignedTransaction.mockResolvedValueOnce(txReceipt);

    const client = createClient();
    const receipt = await client.reportYield(yieldProvider, l2Recipient);

    expect(logger.debug).toHaveBeenCalledWith(
      `reportYield started, yieldProvider=${yieldProvider}, l2YieldRecipient=${l2Recipient}`,
    );
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "reportYield",
      args: [yieldProvider, l2Recipient],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(contractAddress, calldata);
    expect(logger.info).toHaveBeenCalledWith(
      `reportYield succeeded, yieldProvider=${yieldProvider}, l2YieldRecipient=${l2Recipient}, txHash=${txReceipt.transactionHash}`,
    );
    expect(receipt).toBe(txReceipt);
  });

  it("unstakes with encoded withdrawal params and pays validator fee", async () => {
    const withdrawalParams: WithdrawalRequests = {
      pubkeys: ["0x01"] as Hex[],
      amountsGwei: [32n],
    };
    const encodedWithdrawalParams = "0xencoded" as Hex;
    const calldata = "0xunstake" as Hex;
    const fee = 123n;
    const txReceipt = { transactionHash: "0xunstakehash" } as unknown as TransactionReceipt;

    mockedEncodeAbiParameters.mockReturnValueOnce(encodedWithdrawalParams);
    mockedEncodeFunctionData.mockReturnValueOnce(calldata);
    blockchainClient.sendSignedTransaction.mockResolvedValueOnce(txReceipt);
    (publicClient.readContract as jest.Mock).mockResolvedValueOnce(fee);

    const client = createClient();
    const receipt = await client.unstake(yieldProvider, withdrawalParams);

    expect(logger.debug).toHaveBeenCalledWith(`unstake started, yieldProvider=${yieldProvider}`, {
      withdrawalParams,
    });
    expect(mockedEncodeAbiParameters).toHaveBeenCalledWith(
      [
        {
          type: "tuple",
          components: [
            { name: "pubkeys", type: "bytes[]" },
            { name: "amounts", type: "uint64[]" },
            { name: "refundRecipient", type: "address" },
          ],
        },
      ],
      [
        {
          pubkeys: withdrawalParams.pubkeys,
          amounts: withdrawalParams.amountsGwei,
          refundRecipient: contractAddress,
        },
      ],
    );
    expect(publicClient.readContract).toHaveBeenCalledWith({
      address: stakingVaultAddress,
      abi: StakingVaultABI,
      functionName: "calculateValidatorWithdrawalFee",
      args: [BigInt(withdrawalParams.pubkeys.length)],
    });
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "unstake",
      args: [yieldProvider, encodedWithdrawalParams],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(contractAddress, calldata, fee);
    expect(logger.info).toHaveBeenCalledWith(
      `unstake succeeded, yieldProvider=${yieldProvider}, txHash=${txReceipt.transactionHash}`,
      { withdrawalParams },
    );
    expect(receipt).toBe(txReceipt);
  });

  it("adds to withdrawal reserve and logs success", async () => {
    const amount = 55n;
    const calldata = "0xreserve" as Hex;
    const txReceipt = { transactionHash: "0xreservehash" } as unknown as TransactionReceipt;
    mockedEncodeFunctionData.mockReturnValueOnce(calldata);
    blockchainClient.sendSignedTransaction.mockResolvedValueOnce(txReceipt);

    const client = createClient();
    const receipt = await client.safeAddToWithdrawalReserve(yieldProvider, amount);

    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "safeAddToWithdrawalReserve",
      args: [yieldProvider, amount],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(contractAddress, calldata);
    expect(logger.info).toHaveBeenCalledWith(
      `safeAddToWithdrawalReserve succeeded, yieldProvider=${yieldProvider}, amount=${amount.toString()}, txHash=${txReceipt.transactionHash}`,
    );
    expect(receipt).toBe(txReceipt);
  });

  it("pauses and unpauses staking through encoded calls", async () => {
    const pauseCalldata = "0xpause" as Hex;
    const unpauseCalldata = "0xunpause" as Hex;
    const txReceipt = { transactionHash: "0xhash" } as unknown as TransactionReceipt;

    mockedEncodeFunctionData.mockReturnValueOnce(pauseCalldata).mockReturnValueOnce(unpauseCalldata);
    blockchainClient.sendSignedTransaction.mockResolvedValue(txReceipt);

    const client = createClient();

    await client.pauseStaking(yieldProvider);
    expect(mockedEncodeFunctionData).toHaveBeenNthCalledWith(1, {
      abi: contractStub.abi,
      functionName: "pauseStaking",
      args: [yieldProvider],
    });
    expect(logger.info).toHaveBeenCalledWith(
      `pauseStaking succeeded, yieldProvider=${yieldProvider}, txHash=${txReceipt.transactionHash}`,
    );

    await client.unpauseStaking(yieldProvider);
    expect(mockedEncodeFunctionData).toHaveBeenNthCalledWith(2, {
      abi: contractStub.abi,
      functionName: "unpauseStaking",
      args: [yieldProvider],
    });
    expect(logger.info).toHaveBeenCalledWith(
      `unpauseStaking succeeded, yieldProvider=${yieldProvider}, txHash=${txReceipt.transactionHash}`,
    );
  });

  it("progresses pending ossification", async () => {
    const calldata = "0xprogress" as Hex;
    mockedEncodeFunctionData.mockReturnValueOnce(calldata);

    const client = createClient();
    await client.progressPendingOssification(yieldProvider);

    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "progressPendingOssification",
      args: [yieldProvider],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(contractAddress, calldata);
  });

  it("evaluates rebalance requirements with tolerance band", async () => {
    const totalSystemBalance = 1_000_000n;
    const effectiveTarget = 500_000n;
    contractStub.read.getTotalSystemBalance.mockResolvedValue(totalSystemBalance);
    contractStub.read.getEffectiveTargetWithdrawalReserve.mockResolvedValue(effectiveTarget);

    const client = createClient({ rebalanceToleranceBps: 100 });

    // Within tolerance band => no rebalance
    blockchainClient.getBalance.mockResolvedValueOnce(effectiveTarget + 5_000n);
    await expect(client.getRebalanceRequirements()).resolves.toEqual({
      rebalanceDirection: RebalanceDirection.NONE,
      rebalanceAmount: 0n,
    });

    // Deficit => UNSTAKE
    blockchainClient.getBalance.mockResolvedValueOnce(effectiveTarget - 20_000n);
    await expect(client.getRebalanceRequirements()).resolves.toEqual({
      rebalanceDirection: RebalanceDirection.UNSTAKE,
      rebalanceAmount: 20_000n,
    });

    // Surplus => STAKE
    blockchainClient.getBalance.mockResolvedValueOnce(effectiveTarget + 30_000n);
    await expect(client.getRebalanceRequirements()).resolves.toEqual({
      rebalanceDirection: RebalanceDirection.STAKE,
      rebalanceAmount: 30_000n,
    });
  });

  it("returns staking vault and dashboard addresses from yield provider data", async () => {
    contractStub.read.getYieldProviderData.mockResolvedValueOnce({
      ...defaultYieldProviderData,
      primaryEntrypoint: l2Recipient,
      ossifiedEntrypoint: stakingVaultAddress,
    });

    const client = createClient();

    await expect(client.getLidoStakingVaultAddress(yieldProvider)).resolves.toBe(stakingVaultAddress);
    await expect(client.getLidoDashboardAddress(yieldProvider)).resolves.toBe(l2Recipient);
  });

  it("pauses staking only when not already paused", async () => {
    const txReceipt = { transactionHash: "0xpause" } as unknown as TransactionReceipt;
    const client = createClient();
    const pauseSpy = jest.spyOn(client, "pauseStaking").mockResolvedValue(txReceipt);

    contractStub.read.isStakingPaused.mockResolvedValueOnce(false).mockResolvedValueOnce(true);

    await expect(client.pauseStakingIfNotAlready(yieldProvider)).resolves.toBe(txReceipt);
    await expect(client.pauseStakingIfNotAlready(yieldProvider)).resolves.toBeUndefined();
    expect(logger.info).toHaveBeenCalledWith(`Already paused staking for yieldProvider=${yieldProvider}`);
    expect(pauseSpy).toHaveBeenCalledTimes(1);
  });

  it("unpauses staking only when currently paused", async () => {
    const txReceipt = { transactionHash: "0xunpause" } as unknown as TransactionReceipt;
    const client = createClient();
    const unpauseSpy = jest.spyOn(client, "unpauseStaking").mockResolvedValue(txReceipt);

    contractStub.read.isStakingPaused.mockResolvedValueOnce(true).mockResolvedValueOnce(false);

    await expect(client.unpauseStakingIfNotAlready(yieldProvider)).resolves.toBe(txReceipt);
    await expect(client.unpauseStakingIfNotAlready(yieldProvider)).resolves.toBeUndefined();
    expect(logger.info).toHaveBeenCalledWith(`Already resumed staking for yieldProvider=${yieldProvider}`);
    expect(unpauseSpy).toHaveBeenCalledTimes(1);
  });

  it("computes available unstaking rebalance balance", async () => {
    blockchainClient.getBalance.mockResolvedValueOnce(1_000n);
    contractStub.simulate.withdrawableValue.mockResolvedValueOnce({ result: 500n });

    const client = createClient();
    await expect(client.getAvailableUnstakingRebalanceBalance(yieldProvider)).resolves.toBe(1_500n);

    expect(blockchainClient.getBalance).toHaveBeenCalledWith(contractAddress);
    expect(contractStub.simulate.withdrawableValue).toHaveBeenCalledWith([yieldProvider]);
  });

  it("adds to withdrawal reserve only when above threshold", async () => {
    const client = createClient({ minWithdrawalThresholdEth: 1n });
    const addSpy = jest.spyOn(client, "safeAddToWithdrawalReserve").mockResolvedValue(undefined as any);
    jest
      .spyOn(client, "getAvailableUnstakingRebalanceBalance")
      .mockResolvedValueOnce(ONE_ETHER - 1n)
      .mockResolvedValueOnce(ONE_ETHER + 100n);

    await expect(client.safeAddToWithdrawalReserveIfAboveThreshold(yieldProvider, 5n)).resolves.toBeUndefined();
    expect(addSpy).not.toHaveBeenCalled();

    await client.safeAddToWithdrawalReserveIfAboveThreshold(yieldProvider, 7n);
    expect(addSpy).toHaveBeenCalledWith(yieldProvider, 7n);
  });

  it("adds the full available balance when calling safeMaxAddToWithdrawalReserve", async () => {
    const client = createClient({ minWithdrawalThresholdEth: 1n });
    const addSpy = jest.spyOn(client, "safeAddToWithdrawalReserve").mockResolvedValue(undefined as any);
    const available = ONE_ETHER + 50n;
    jest.spyOn(client, "getAvailableUnstakingRebalanceBalance").mockResolvedValue(available);

    await client.safeMaxAddToWithdrawalReserve(yieldProvider);

    expect(addSpy).toHaveBeenCalledWith(yieldProvider, available);
  });

  it("skips safeMaxAddToWithdrawalReserve when below the threshold", async () => {
    const client = createClient({ minWithdrawalThresholdEth: 2n });
    const addSpy = jest.spyOn(client, "safeAddToWithdrawalReserve").mockResolvedValue(undefined as any);
    jest.spyOn(client, "getAvailableUnstakingRebalanceBalance").mockResolvedValue(2n * ONE_ETHER - 1n);

    await expect(client.safeMaxAddToWithdrawalReserve(yieldProvider)).resolves.toBeUndefined();
    expect(addSpy).not.toHaveBeenCalled();
  });

  it("extracts withdrawal events from receipts emitted by the contract", () => {
    const client = createClient();
    const log = { address: contractAddress, data: "0xdata", topics: ["0x01"] };
    mockedParseEventLogs.mockReturnValueOnce([
      {
        eventName: "WithdrawalReserveAugmented",
        args: { reserveIncrementAmount: 10n, yieldProvider },
        address: contractAddress,
      } as any,
    ]);

    const event = client.getWithdrawalEventFromTxReceipt(buildReceipt([log]));

    expect(event).toEqual({ reserveIncrementAmount: 10n, yieldProvider });
    expect(mockedParseEventLogs).toHaveBeenCalledWith({
      abi: contractStub.abi,
      eventName: "WithdrawalReserveAugmented",
      logs: [log],
    });
  });

  it("returns undefined when withdrawal events are absent or decoding fails", () => {
    const client = createClient();
    mockedParseEventLogs.mockReturnValueOnce([]);

    const event = client.getWithdrawalEventFromTxReceipt(
      buildReceipt([{ address: contractAddress.toUpperCase(), data: "0x", topics: [] }]),
    );

    expect(event).toBeUndefined();
    expect(mockedParseEventLogs).toHaveBeenCalledTimes(1);
  });

  it("ignores withdrawal events from other contracts", () => {
    const client = createClient();
    const foreignLog = { address: "0x1234567890123456789012345678901234567890", data: "0x", topics: [] };

    mockedParseEventLogs.mockReturnValueOnce([
      {
        eventName: "WithdrawalReserveAugmented",
        args: { reserveIncrementAmount: 10n, yieldProvider },
        address: "0x1234567890123456789012345678901234567890",
      } as any,
    ]);

    const event = client.getWithdrawalEventFromTxReceipt(buildReceipt([foreignLog]));

    expect(event).toBeUndefined();
    expect(mockedParseEventLogs).toHaveBeenCalledTimes(1);
  });

  it("extracts yield reports from receipts emitted by the contract", () => {
    const client = createClient();
    const log = { address: contractAddress, data: "0xfeed", topics: ["0x1111"] };
    mockedParseEventLogs.mockReturnValueOnce([
      {
        eventName: "NativeYieldReported",
        args: { yieldAmount: 12n, outstandingNegativeYield: 5n, yieldProvider },
        address: contractAddress,
      } as any,
    ]);

    const report = client.getYieldReportFromTxReceipt(buildReceipt([log]));

    expect(report).toEqual({ yieldAmount: 12n, outstandingNegativeYield: 5n, yieldProvider });
    expect(mockedParseEventLogs).toHaveBeenCalledWith({
      abi: contractStub.abi,
      eventName: "NativeYieldReported",
      logs: [log],
    });
  });

  it("returns undefined when yield report events are absent", () => {
    const client = createClient();
    mockedParseEventLogs.mockReturnValueOnce([]);

    const report = client.getYieldReportFromTxReceipt(
      buildReceipt([{ address: contractAddress, data: "0x0", topics: [] }]),
    );

    expect(report).toBeUndefined();
  });

  it("ignores yield report logs from other contracts", () => {
    const client = createClient();
    const foreignLog = { address: "0x1234567890123456789012345678901234567890", data: "0x", topics: [] };

    mockedParseEventLogs.mockReturnValueOnce([
      {
        eventName: "NativeYieldReported",
        args: { yieldAmount: 12n, outstandingNegativeYield: 5n, yieldProvider },
        address: "0x1234567890123456789012345678901234567890",
      } as any,
    ]);

    const report = client.getYieldReportFromTxReceipt(buildReceipt([foreignLog]));

    expect(report).toBeUndefined();
    expect(mockedParseEventLogs).toHaveBeenCalledTimes(1);
  });
});
