import { mock, MockProxy } from "jest-mock-extended";
import type { ILogger, IBlockchainClient } from "@consensys/linea-shared-utils";
import { absDiff, ONE_ETHER, weiToGweiNumber } from "@consensys/linea-shared-utils";
import type { Address, Hex, PublicClient, TransactionReceipt } from "viem";
import { YieldManagerABI } from "../../../core/abis/YieldManager.js";
import { StakingVaultABI } from "../../../core/abis/StakingVault.js";
import { DashboardErrorsABI } from "../../../core/abis/errors/DashboardErrors.js";
import { LidoStVaultYieldProviderErrorsABI } from "../../../core/abis/errors/LidoStVaultYieldProviderErrors.js";
import { StakingVaultErrorsABI } from "../../../core/abis/errors/StakingVaultErrors.js";
import { VaultHubErrorsABI } from "../../../core/abis/errors/VaultHubErrors.js";
import { RebalanceDirection } from "../../../core/entities/RebalanceRequirement.js";
import type { WithdrawalRequests } from "../../../core/entities/LidoStakingVaultWithdrawalParams.js";
import type { INativeYieldAutomationMetricsUpdater } from "../../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { IRebalanceQuotaService } from "../../../core/services/IRebalanceQuotaService.js";

const YieldManagerCombinedABI = [
  ...YieldManagerABI,
  ...DashboardErrorsABI,
  ...LidoStVaultYieldProviderErrorsABI,
  ...StakingVaultErrorsABI,
  ...VaultHubErrorsABI,
] as const;

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

import { concat, getContract, encodeFunctionData, parseEventLogs, encodeAbiParameters } from "viem";

jest.mock("../DashboardContractClient.js", () => ({
  DashboardContractClient: {
    getOrCreate: jest.fn(),
    initialize: jest.fn(),
  },
}));

import { DashboardContractClient } from "../DashboardContractClient.js";

const mockedGetContract = getContract as jest.MockedFunction<typeof getContract>;
const mockedEncodeFunctionData = encodeFunctionData as jest.MockedFunction<typeof encodeFunctionData>;
const mockedParseEventLogs = parseEventLogs as jest.MockedFunction<typeof parseEventLogs>;
const mockedEncodeAbiParameters = encodeAbiParameters as jest.MockedFunction<typeof encodeAbiParameters>;
const mockedDashboardContractClientGetOrCreate = DashboardContractClient.getOrCreate as jest.MockedFunction<
  typeof DashboardContractClient.getOrCreate
>;

let YieldManagerContractClient: typeof import("../YieldManagerContractClient.js").YieldManagerContractClient;

beforeAll(async () => {
  ({ YieldManagerContractClient } = await import("../YieldManagerContractClient.js"));
});

describe("YieldManagerContractClient", () => {
  // Semantic constants
  const CONTRACT_ADDRESS = "0xcccccccccccccccccccccccccccccccccccccccc" as Address;
  const YIELD_PROVIDER = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" as Address;
  const L2_RECIPIENT = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" as Address;
  const L1_MESSAGE_SERVICE_ADDRESS = "0x9999999999999999999999999999999999999999" as Address;
  const STAKING_VAULT_ADDRESS = "0x8888888888888888888888888888888888888888" as Address;
  const SIGNER_ADDRESS = "0xdddddddddddddddddddddddddddddddddddddddd" as Address;
  const ONE_ETH = 1_000_000_000_000_000_000n;
  const DEFAULT_REBALANCE_TOLERANCE = ONE_ETH;

  let logger: MockProxy<ILogger>;
  let blockchainClient: MockProxy<IBlockchainClient<PublicClient, TransactionReceipt>>;
  let publicClient: { readContract: jest.Mock } & Record<string, unknown>;
  let contractStub: {
    abi: typeof YieldManagerABI;
    read: Record<string, jest.Mock>;
    simulate: Record<string, jest.Mock>;
  };

  // Factory functions for test data
  const createYieldProviderData = (overrides = {}) => ({
    yieldProviderVendor: 0,
    isStakingPaused: false,
    isOssificationInitiated: false,
    isOssified: false,
    primaryEntrypoint: L2_RECIPIENT,
    ossifiedEntrypoint: STAKING_VAULT_ADDRESS,
    yieldProviderIndex: 0n,
    userFunds: 0n,
    yieldReportedCumulative: 0n,
    lstLiabilityPrincipal: 0n,
    lastReportedNegativeYield: 0n,
    ...overrides,
  });

  const createTransactionReceipt = (logs: Array<{ address: string; data: string; topics: string[] }>): TransactionReceipt =>
    ({
      logs,
    }) as unknown as TransactionReceipt;

  const createDefaultQuotaService = (): IRebalanceQuotaService => ({
    getRebalanceAmountAfterQuota: jest.fn((_vaultAddress, _totalSystemBalance, reBalanceAmountWei) => reBalanceAmountWei),
    getStakingDirection: jest.fn(() => RebalanceDirection.STAKE),
  });

  const createClient = ({
    rebalanceToleranceAmountWei = DEFAULT_REBALANCE_TOLERANCE,
    minWithdrawalThresholdEth = 0n,
    rebalanceQuotaService = createDefaultQuotaService(),
    metricsUpdater,
  }: {
    rebalanceToleranceAmountWei?: bigint;
    minWithdrawalThresholdEth?: bigint;
    rebalanceQuotaService?: IRebalanceQuotaService;
    metricsUpdater?: Partial<INativeYieldAutomationMetricsUpdater>;
  } = {}) =>
    new YieldManagerContractClient(
      logger,
      blockchainClient,
      CONTRACT_ADDRESS,
      rebalanceToleranceAmountWei,
      minWithdrawalThresholdEth,
      rebalanceQuotaService,
      metricsUpdater as INativeYieldAutomationMetricsUpdater | undefined,
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
        getYieldProviderData: jest.fn().mockResolvedValue(createYieldProviderData()),
        userFunds: jest.fn(),
      },
      simulate: {
        withdrawableValue: jest.fn(),
        reportYield: jest.fn(),
      },
    };

    mockedGetContract.mockReturnValue(contractStub as any);
    blockchainClient.getBalance.mockResolvedValue(0n);
    blockchainClient.getSignerAddress.mockReturnValue(SIGNER_ADDRESS);
    blockchainClient.sendSignedTransaction.mockResolvedValue({
      transactionHash: "0xreceipt",
    } as unknown as TransactionReceipt);
    contractStub.read.L1_MESSAGE_SERVICE.mockResolvedValue(L1_MESSAGE_SERVICE_ADDRESS);
    contractStub.read.getTotalSystemBalance.mockResolvedValue(0n);
    contractStub.read.getEffectiveTargetWithdrawalReserve.mockResolvedValue(0n);
    contractStub.read.getTargetReserveDeficit.mockResolvedValue(0n);
    contractStub.read.isStakingPaused.mockResolvedValue(false);
    contractStub.read.isOssificationInitiated.mockResolvedValue(false);
    contractStub.read.isOssified.mockResolvedValue(false);
    contractStub.simulate.withdrawableValue.mockResolvedValue({ result: 0n });
  });

  it("initializes the viem contract and exposes address and contract accessors", () => {
    // Arrange
    // (setup in beforeEach)

    // Act
    const client = createClient();

    // Assert
    expect(mockedGetContract).toHaveBeenCalledWith({
      abi: YieldManagerCombinedABI,
      address: CONTRACT_ADDRESS,
      client: publicClient,
    });
    expect(client.getAddress()).toBe(CONTRACT_ADDRESS);
    expect(client.getContract()).toBe(contractStub);
  });

  it("gets the contract balance", async () => {
    // Arrange
    const balance = ONE_ETH;
    blockchainClient.getBalance.mockResolvedValueOnce(balance);
    const client = createClient();

    // Act
    const result = await client.getBalance();

    // Assert
    expect(blockchainClient.getBalance).toHaveBeenCalledWith(CONTRACT_ADDRESS);
    expect(result).toBe(balance);
  });

  it("delegates simple reads to the viem contract", async () => {
    // Arrange
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

    // Act
    const totalSystemBalance = await client.getTotalSystemBalance();
    const effectiveTarget = await client.getEffectiveTargetWithdrawalReserve();
    const targetDeficit = await client.getTargetReserveDeficit();
    const l1MessageService = await client.L1_MESSAGE_SERVICE();
    const isPaused = await client.isStakingPaused(YIELD_PROVIDER);
    const isOssifying = await client.isOssificationInitiated(YIELD_PROVIDER);
    const isOssifiedResult = await client.isOssified(YIELD_PROVIDER);

    // Assert
    expect(totalSystemBalance).toBe(123n);
    expect(effectiveTarget).toBe(456n);
    expect(targetDeficit).toBe(789n);
    expect(l1MessageService).toBe(L1_MESSAGE_SERVICE_ADDRESS);
    expect(isPaused).toBe(stakingPaused);
    expect(isOssifying).toBe(ossificationInitiated);
    expect(isOssifiedResult).toBe(ossified);
    expect(contractStub.read.getTotalSystemBalance).toHaveBeenCalledTimes(1);
    expect(contractStub.read.getEffectiveTargetWithdrawalReserve).toHaveBeenCalledTimes(1);
    expect(contractStub.read.getTargetReserveDeficit).toHaveBeenCalledTimes(1);
    expect(contractStub.read.L1_MESSAGE_SERVICE).toHaveBeenCalledTimes(1);
    expect(contractStub.read.isStakingPaused).toHaveBeenCalledWith([YIELD_PROVIDER]);
    expect(contractStub.read.isOssificationInitiated).toHaveBeenCalledWith([YIELD_PROVIDER]);
    expect(contractStub.read.isOssified).toHaveBeenCalledWith([YIELD_PROVIDER]);
  });

  it("reads withdrawable value via simulate and returns the result", async () => {
    // Arrange
    const withdrawable = 42n;
    contractStub.simulate.withdrawableValue.mockResolvedValueOnce({ result: withdrawable });
    const client = createClient();

    // Act
    const result = await client.withdrawableValue(YIELD_PROVIDER);

    // Assert
    expect(contractStub.simulate.withdrawableValue).toHaveBeenCalledWith([YIELD_PROVIDER]);
    expect(result).toBe(withdrawable);
  });

  it("peeks yield report via simulate and returns mapped YieldReport", async () => {
    // Arrange
    const newReportedYield = 100n;
    const outstandingNegativeYield = 25n;
    contractStub.simulate.reportYield.mockResolvedValueOnce({
      result: [newReportedYield, outstandingNegativeYield],
    });
    const client = createClient();

    // Act
    const result = await client.peekYieldReport(YIELD_PROVIDER, L2_RECIPIENT);

    // Assert
    expect(contractStub.simulate.reportYield).toHaveBeenCalledWith([YIELD_PROVIDER, L2_RECIPIENT], {
      account: SIGNER_ADDRESS,
    });
    expect(result).toEqual({
      yieldAmount: newReportedYield,
      outstandingNegativeYield,
      yieldProvider: YIELD_PROVIDER,
    });
  });

  it("returns undefined when peekYieldReport simulation fails", async () => {
    // Arrange
    contractStub.simulate.reportYield.mockRejectedValueOnce(new Error("Simulation failed"));
    const client = createClient();

    // Act
    const result = await client.peekYieldReport(YIELD_PROVIDER, L2_RECIPIENT);

    // Assert
    expect(contractStub.simulate.reportYield).toHaveBeenCalledWith([YIELD_PROVIDER, L2_RECIPIENT], {
      account: SIGNER_ADDRESS,
    });
    expect(result).toBeUndefined();
    expect(logger.error).toHaveBeenCalledWith(
      `peekYieldReport failed, yieldProvider=${YIELD_PROVIDER}, l2YieldRecipient=${L2_RECIPIENT}`,
      { error: expect.any(Error) },
    );
  });

  it("encodes calldata and sends fundYieldProvider transaction", async () => {
    // Arrange
    const amount = 100n;
    const calldata = "0xcalldata" as Hex;
    const txReceipt = { transactionHash: "0xhash" } as unknown as TransactionReceipt;
    mockedEncodeFunctionData.mockReturnValueOnce(calldata);
    blockchainClient.sendSignedTransaction.mockResolvedValueOnce(txReceipt);
    const client = createClient();

    // Act
    const receipt = await client.fundYieldProvider(YIELD_PROVIDER, amount);

    // Assert
    expect(logger.info).toHaveBeenCalledWith(
      `fundYieldProvider started, yieldProvider=${YIELD_PROVIDER}, amount=${amount.toString()}`,
    );
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "fundYieldProvider",
      args: [YIELD_PROVIDER, amount],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(
      CONTRACT_ADDRESS,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
    expect(logger.info).toHaveBeenCalledWith(
      `fundYieldProvider succeeded, yieldProvider=${YIELD_PROVIDER}, amount=${amount.toString()}, txHash=${txReceipt.transactionHash}`,
    );
    expect(receipt).toBe(txReceipt);
  });

  it("reports yield with encoded calldata", async () => {
    // Arrange
    const calldata = "0xreport" as Hex;
    const txReceipt = { transactionHash: "0xreporthash" } as unknown as TransactionReceipt;
    mockedEncodeFunctionData.mockReturnValueOnce(calldata);
    blockchainClient.sendSignedTransaction.mockResolvedValueOnce(txReceipt);
    const client = createClient();

    // Act
    const receipt = await client.reportYield(YIELD_PROVIDER, L2_RECIPIENT);

    // Assert
    expect(logger.info).toHaveBeenCalledWith(
      `reportYield started, yieldProvider=${YIELD_PROVIDER}, l2YieldRecipient=${L2_RECIPIENT}`,
    );
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "reportYield",
      args: [YIELD_PROVIDER, L2_RECIPIENT],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(
      CONTRACT_ADDRESS,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
    expect(logger.info).toHaveBeenCalledWith(
      `reportYield succeeded, yieldProvider=${YIELD_PROVIDER}, l2YieldRecipient=${L2_RECIPIENT}, txHash=${txReceipt.transactionHash}`,
    );
    expect(receipt).toBe(txReceipt);
  });

  it("unstakes with encoded withdrawal params and pays validator fee", async () => {
    // Arrange
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

    // Act
    const receipt = await client.unstake(YIELD_PROVIDER, withdrawalParams);

    // Assert
    expect(logger.info).toHaveBeenCalledWith(
      `unstake started, yieldProvider=${YIELD_PROVIDER}, validatorCount=${withdrawalParams.pubkeys.length}`,
    );
    expect(logger.debug).toHaveBeenCalledWith(`unstake started withdrawalParams`, {
      withdrawalParams,
    });
    expect(mockedEncodeAbiParameters).toHaveBeenCalledWith(
      [
        { name: "pubkeys", type: "bytes" },
        { name: "amounts", type: "uint64[]" },
        { name: "refundRecipient", type: "address" },
      ],
      [concat(withdrawalParams.pubkeys), withdrawalParams.amountsGwei, CONTRACT_ADDRESS],
    );
    expect(publicClient.readContract).toHaveBeenCalledWith({
      address: STAKING_VAULT_ADDRESS,
      abi: StakingVaultABI,
      functionName: "calculateValidatorWithdrawalFee",
      args: [BigInt(withdrawalParams.pubkeys.length)],
    });
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "unstake",
      args: [YIELD_PROVIDER, encodedWithdrawalParams],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(
      CONTRACT_ADDRESS,
      calldata,
      fee,
      YieldManagerCombinedABI,
    );
    expect(logger.info).toHaveBeenCalledWith(
      `unstake succeeded, yieldProvider=${YIELD_PROVIDER}, validatorCount=${withdrawalParams.pubkeys.length}, txHash=${txReceipt.transactionHash}`,
    );
    expect(logger.debug).toHaveBeenCalledWith(`unstake succeeded withdrawalParams`, {
      withdrawalParams,
    });
    expect(receipt).toBe(txReceipt);
  });

  it("adds to withdrawal reserve and logs success", async () => {
    // Arrange
    const amount = 55n;
    const calldata = "0xreserve" as Hex;
    const txReceipt = { transactionHash: "0xreservehash" } as unknown as TransactionReceipt;
    mockedEncodeFunctionData.mockReturnValueOnce(calldata);
    blockchainClient.sendSignedTransaction.mockResolvedValueOnce(txReceipt);
    const client = createClient();

    // Act
    const receipt = await client.safeAddToWithdrawalReserve(YIELD_PROVIDER, amount);

    // Assert
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "safeAddToWithdrawalReserve",
      args: [YIELD_PROVIDER, amount],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(
      CONTRACT_ADDRESS,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
    expect(logger.info).toHaveBeenCalledWith(
      `safeAddToWithdrawalReserve succeeded, yieldProvider=${YIELD_PROVIDER}, amount=${amount.toString()}, txHash=${txReceipt.transactionHash}`,
    );
    expect(receipt).toBe(txReceipt);
  });

  it("withdraws from yield provider and logs success", async () => {
    // Arrange
    const amount = 75n;
    const calldata = "0xwithdraw" as Hex;
    const txReceipt = { transactionHash: "0xwithdrawhash" } as unknown as TransactionReceipt;
    mockedEncodeFunctionData.mockReturnValueOnce(calldata);
    blockchainClient.sendSignedTransaction.mockResolvedValueOnce(txReceipt);
    const client = createClient();

    // Act
    const receipt = await client.safeWithdrawFromYieldProvider(YIELD_PROVIDER, amount);

    // Assert
    expect(logger.info).toHaveBeenCalledWith(
      `safeWithdrawFromYieldProvider started, yieldProvider=${YIELD_PROVIDER}, amount=${amount.toString()}`,
    );
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "safeWithdrawFromYieldProvider",
      args: [YIELD_PROVIDER, amount],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(
      CONTRACT_ADDRESS,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
    expect(logger.info).toHaveBeenCalledWith(
      `safeWithdrawFromYieldProvider succeeded, yieldProvider=${YIELD_PROVIDER}, amount=${amount.toString()}, txHash=${txReceipt.transactionHash}`,
    );
    expect(receipt).toBe(txReceipt);
  });

  it("pauses staking through encoded call", async () => {
    // Arrange
    const pauseCalldata = "0xpause" as Hex;
    const txReceipt = { transactionHash: "0xhash" } as unknown as TransactionReceipt;
    mockedEncodeFunctionData.mockReturnValueOnce(pauseCalldata);
    blockchainClient.sendSignedTransaction.mockResolvedValue(txReceipt);
    const client = createClient();

    // Act
    await client.pauseStaking(YIELD_PROVIDER);

    // Assert
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "pauseStaking",
      args: [YIELD_PROVIDER],
    });
    expect(logger.info).toHaveBeenCalledWith(
      `pauseStaking succeeded, yieldProvider=${YIELD_PROVIDER}, txHash=${txReceipt.transactionHash}`,
    );
  });

  it("unpauses staking through encoded call", async () => {
    // Arrange
    const unpauseCalldata = "0xunpause" as Hex;
    const txReceipt = { transactionHash: "0xhash" } as unknown as TransactionReceipt;
    mockedEncodeFunctionData.mockReturnValueOnce(unpauseCalldata);
    blockchainClient.sendSignedTransaction.mockResolvedValue(txReceipt);
    const client = createClient();

    // Act
    await client.unpauseStaking(YIELD_PROVIDER);

    // Assert
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "unpauseStaking",
      args: [YIELD_PROVIDER],
    });
    expect(logger.info).toHaveBeenCalledWith(
      `unpauseStaking succeeded, yieldProvider=${YIELD_PROVIDER}, txHash=${txReceipt.transactionHash}`,
    );
  });

  it("progresses pending ossification", async () => {
    // Arrange
    const calldata = "0xprogress" as Hex;
    mockedEncodeFunctionData.mockReturnValueOnce(calldata);
    const client = createClient();

    // Act
    await client.progressPendingOssification(YIELD_PROVIDER);

    // Assert
    expect(mockedEncodeFunctionData).toHaveBeenCalledWith({
      abi: contractStub.abi,
      functionName: "progressPendingOssification",
      args: [YIELD_PROVIDER],
    });
    expect(blockchainClient.sendSignedTransaction).toHaveBeenCalledWith(
      CONTRACT_ADDRESS,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
  });

  // ⚠️ N.B. — WARNING: Below describe block tests were intentionally handwritten because
  // the rebalance logic is too critical (and too fragile) to trust to vibe-coding.
  // Do NOT vibe-code away unless you fully understand the consequences.
  describe("getRebalanceRequirements", () => {
    /**
     * Helper to setup all mocks for a rebalance test scenario.
     */
    function setupRebalanceTest(
      l1MessageServiceBalance: bigint,
      totalSystemBalance: bigint,
      effectiveTarget: bigint,
      dashboardTotalValue: bigint,
      userFunds: bigint,
      peekedYieldAmount: bigint,
      peekedOutstandingNegativeYield: bigint,
      rebalanceToleranceAmountWei: bigint = DEFAULT_REBALANCE_TOLERANCE,
      rebalanceQuotaService?: IRebalanceQuotaService,
      metricsUpdater?: Partial<INativeYieldAutomationMetricsUpdater>,
    ) {
      // Arrange
      contractStub.read.getTotalSystemBalance.mockResolvedValue(totalSystemBalance);
      contractStub.read.getEffectiveTargetWithdrawalReserve.mockResolvedValue(effectiveTarget);
      contractStub.read.userFunds.mockResolvedValue(userFunds);

      contractStub.simulate.reportYield.mockResolvedValue({
        result: [peekedYieldAmount, peekedOutstandingNegativeYield],
      });

      const mockDashboardClient = {
        totalValue: jest.fn().mockResolvedValue(dashboardTotalValue),
      };
      mockedDashboardContractClientGetOrCreate.mockReturnValue(mockDashboardClient as any);

      blockchainClient.getBalance.mockResolvedValueOnce(l1MessageServiceBalance);

      const client = createClient({
        rebalanceToleranceAmountWei,
        rebalanceQuotaService,
        metricsUpdater: metricsUpdater as INativeYieldAutomationMetricsUpdater | undefined,
      });

      return {
        client,
        mockDashboardClient,
        contractStub,
        blockchainClient,
      };
    }

    it("throws error when peekYieldReport returns undefined", async () => {
      // Arrange
      contractStub.read.getTotalSystemBalance.mockResolvedValue(1_000_000n);
      contractStub.read.getEffectiveTargetWithdrawalReserve.mockResolvedValue(500_000n);
      contractStub.read.userFunds.mockResolvedValue(100_000n);
      contractStub.simulate.reportYield.mockResolvedValue(undefined);
      const mockDashboardClient = {
        totalValue: jest.fn().mockResolvedValue(600_000n),
      };
      mockedDashboardContractClientGetOrCreate.mockReturnValue(mockDashboardClient as any);
      const client = createClient();

      // Act & Assert
      await expect(client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT)).rejects.toThrow(
        "peekYieldReport returned undefined, cannot determine rebalance requirements",
      );
    });

    it("returns NONE when absRebalanceRequirement is within tolerance band lower bound", async () => {
      // Arrange
      const l1MessageServiceBalance = 490_001n;
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 500_000n;
      const dashboardTotalValue = 500_000n;
      const userFunds = 500_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.NONE,
        rebalanceAmount: 0n,
      });
    });

    it("returns NONE when absRebalanceRequirement is within tolerance band upper bound", async () => {
      // Arrange
      const l1MessageServiceBalance = 509_999n;
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 500_000n;
      const dashboardTotalValue = 500_000n;
      const userFunds = 500_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.NONE,
        rebalanceAmount: 0n,
      });
    });

    it("correctly evaluates rebalance requirements for negative yield scenario with system obligations", async () => {
      // Arrange
      const l1MessageServiceBalance = 500_000n;
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 500_000n;
      const dashboardTotalValue = 500_000n;
      const userFunds = 500_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 100_000n;
      const rebalanceToleranceAmountWei = 10_000n;
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.STAKE,
        rebalanceAmount: 150_000n,
      });
    });

    it("correctly evaluates rebalance requirements for negative yield scenario with different balance", async () => {
      // Arrange
      const l1MessageServiceBalance = 450_000n;
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 500_000n;
      const dashboardTotalValue = 500_000n;
      const userFunds = 500_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 100_000n;
      const rebalanceToleranceAmountWei = 10_000n;
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.STAKE,
        rebalanceAmount: 100_000n,
      });
    });

    it("correctly evaluates rebalance requirements for positive yield scenario", async () => {
      // Arrange
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 200_000n;
      const dashboardTotalValue = 600_000n;
      const userFunds = 500_000n;
      const peekedYieldAmount = 50_000n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 50_000n;
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.UNSTAKE,
        rebalanceAmount: 190_000n,
      });
    });

    it("correctly evaluates rebalance requirements for whale depletion scenario", async () => {
      // Arrange
      const totalSystemBalance = 10_000_000n;
      const effectiveTarget = 4_000_000n;
      const dashboardTotalValue = 10_000_000n;
      const userFunds = 10_000_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 5_000_000n;
      const rebalanceToleranceAmountWei = 250_000n;
      const l1MessageServiceBalance = 0n;
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.UNSTAKE,
        rebalanceAmount: 7_000_000n,
      });
    });

    it("caps staking rebalance amount when quota service returns partial amount", async () => {
      // Arrange
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 400_000n;
      const dashboardTotalValue = 100_000n;
      const userFunds = 100_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 900_000n;
      const absRebalanceRequirement = l1MessageServiceBalance - effectiveTarget;
      const cappedAmount = 100_000n;
      const mockQuotaService = createDefaultQuotaService();
      (mockQuotaService.getRebalanceAmountAfterQuota as jest.Mock).mockReturnValue(cappedAmount);
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
        mockQuotaService,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.STAKE,
        rebalanceAmount: cappedAmount,
      });
      expect(mockQuotaService.getRebalanceAmountAfterQuota).toHaveBeenCalledWith(
        STAKING_VAULT_ADDRESS,
        totalSystemBalance,
        absRebalanceRequirement,
      );
    });

    it("correctly applies quota when quota service returns 0n", async () => {
      // Arrange
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 400_000n;
      const dashboardTotalValue = 100_000n;
      const userFunds = 100_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 900_000n;
      const absRebalanceRequirement = l1MessageServiceBalance - effectiveTarget;
      const mockQuotaService = createDefaultQuotaService();
      (mockQuotaService.getRebalanceAmountAfterQuota as jest.Mock).mockReturnValue(0n);
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
        mockQuotaService,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.NONE,
        rebalanceAmount: 0n,
      });
      expect(mockQuotaService.getRebalanceAmountAfterQuota).toHaveBeenCalledWith(
        STAKING_VAULT_ADDRESS,
        totalSystemBalance,
        absRebalanceRequirement,
      );
    });

    it("caps staking amount when absRebalanceAmountAfterQuota exceeds stakingRebalanceCeiling", async () => {
      // Arrange
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 400_000n;
      const dashboardTotalValue = 100_000n;
      const userFunds = 100_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 1_000_000n;
      const absRebalanceAmountAfterQuota = 800_000n;
      const mockQuotaService = createDefaultQuotaService();
      (mockQuotaService.getRebalanceAmountAfterQuota as jest.Mock).mockReturnValue(absRebalanceAmountAfterQuota);
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
        mockQuotaService,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.STAKE,
        rebalanceAmount: 600_000n,
      });
    });

    it("uses full absRebalanceAmountAfterQuota when below stakingRebalanceCeiling", async () => {
      // Arrange
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 400_000n;
      const dashboardTotalValue = 100_000n;
      const userFunds = 100_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 1_000_000n;
      const absRebalanceAmountAfterQuota = 300_000n;
      const mockQuotaService = createDefaultQuotaService();
      (mockQuotaService.getRebalanceAmountAfterQuota as jest.Mock).mockReturnValue(absRebalanceAmountAfterQuota);
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
        mockQuotaService,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.STAKE,
        rebalanceAmount: 300_000n,
      });
    });

    it("handles edge case where stakingRebalanceCeiling equals absRebalanceAmountAfterQuota", async () => {
      // Arrange
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 400_000n;
      const dashboardTotalValue = 100_000n;
      const userFunds = 100_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 1_000_000n;
      const absRebalanceAmountAfterQuota = 600_000n;
      const mockQuotaService = createDefaultQuotaService();
      (mockQuotaService.getRebalanceAmountAfterQuota as jest.Mock).mockReturnValue(absRebalanceAmountAfterQuota);
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
        mockQuotaService,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.STAKE,
        rebalanceAmount: 600_000n,
      });
    });

    it("handles small stakingRebalanceCeiling edge case", async () => {
      // Arrange
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 400_000n;
      const dashboardTotalValue = 100_000n;
      const userFunds = 100_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 400_100n;
      const absRebalanceAmountAfterQuota = 50_000n;
      const mockQuotaService = createDefaultQuotaService();
      (mockQuotaService.getRebalanceAmountAfterQuota as jest.Mock).mockReturnValue(absRebalanceAmountAfterQuota);
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
        mockQuotaService,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.STAKE,
        rebalanceAmount: 100n,
      });
    });

    it("tracks metrics when metricsUpdater is provided and quota service returns 0n", async () => {
      // Arrange
      const metricsUpdater = {
        incrementStakingDepositQuotaExceeded: jest.fn(),
        setActualRebalanceRequirement: jest.fn(),
        setReportedRebalanceRequirement: jest.fn(),
        incrementContractEstimateGasError: jest.fn(),
      };
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 400_000n;
      const dashboardTotalValue = 100_000n;
      const userFunds = 100_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 900_000n;
      const absRebalanceRequirement = l1MessageServiceBalance - effectiveTarget;
      const mockQuotaService = createDefaultQuotaService();
      (mockQuotaService.getRebalanceAmountAfterQuota as jest.Mock).mockReturnValue(0n);
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
        mockQuotaService,
        metricsUpdater,
      );

      // Act
      await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(metricsUpdater.setActualRebalanceRequirement).toHaveBeenCalledWith(
        STAKING_VAULT_ADDRESS,
        weiToGweiNumber(absRebalanceRequirement),
        RebalanceDirection.STAKE,
      );
      expect(metricsUpdater.setReportedRebalanceRequirement).toHaveBeenCalledWith(
        STAKING_VAULT_ADDRESS,
        0,
        RebalanceDirection.NONE,
      );
    });

    it("tracks gauge metric for all rebalance paths when metricsUpdater is provided", async () => {
      // Arrange
      const metricsUpdater = {
        incrementStakingDepositQuotaExceeded: jest.fn(),
        setActualRebalanceRequirement: jest.fn(),
        setReportedRebalanceRequirement: jest.fn(),
        incrementContractEstimateGasError: jest.fn(),
      };
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 400_000n;
      const dashboardTotalValue = 100_000n;
      const userFunds = 100_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 450_000n;
      const absRebalanceRequirement = absDiff(l1MessageServiceBalance, effectiveTarget);
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
        undefined,
        metricsUpdater,
      );

      // Act
      await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(metricsUpdater.setActualRebalanceRequirement).toHaveBeenCalledWith(
        STAKING_VAULT_ADDRESS,
        weiToGweiNumber(absRebalanceRequirement),
        RebalanceDirection.STAKE,
      );
      expect(metricsUpdater.setReportedRebalanceRequirement).toHaveBeenCalledWith(
        STAKING_VAULT_ADDRESS,
        0,
        RebalanceDirection.STAKE,
      );
      expect(metricsUpdater.incrementStakingDepositQuotaExceeded).not.toHaveBeenCalled();
    });

    it("tracks reported requirement for UNSTAKE path when metricsUpdater is provided", async () => {
      // Arrange
      const metricsUpdater = {
        incrementStakingDepositQuotaExceeded: jest.fn(),
        setActualRebalanceRequirement: jest.fn(),
        setReportedRebalanceRequirement: jest.fn(),
        incrementContractEstimateGasError: jest.fn(),
      };
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 400_000n;
      const dashboardTotalValue = 100_000n;
      const userFunds = 100_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 300_000n;
      const absRebalanceRequirement = effectiveTarget - l1MessageServiceBalance;
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
        undefined,
        metricsUpdater,
      );

      // Act
      await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(metricsUpdater.setActualRebalanceRequirement).toHaveBeenCalledWith(
        STAKING_VAULT_ADDRESS,
        weiToGweiNumber(absRebalanceRequirement),
        RebalanceDirection.UNSTAKE,
      );
      expect(metricsUpdater.setReportedRebalanceRequirement).toHaveBeenCalledWith(
        STAKING_VAULT_ADDRESS,
        weiToGweiNumber(absRebalanceRequirement),
        RebalanceDirection.UNSTAKE,
      );
    });

    it("tracks reported requirement for STAKE path when metricsUpdater is provided", async () => {
      // Arrange
      const metricsUpdater = {
        incrementStakingDepositQuotaExceeded: jest.fn(),
        setActualRebalanceRequirement: jest.fn(),
        setReportedRebalanceRequirement: jest.fn(),
        incrementContractEstimateGasError: jest.fn(),
      };
      const totalSystemBalance = 1_000_000n;
      const effectiveTarget = 400_000n;
      const dashboardTotalValue = 100_000n;
      const userFunds = 100_000n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 500_000n;
      const absRebalanceRequirement = l1MessageServiceBalance - effectiveTarget;
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
        undefined,
        metricsUpdater,
      );

      // Act
      await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(metricsUpdater.setActualRebalanceRequirement).toHaveBeenCalledWith(
        STAKING_VAULT_ADDRESS,
        weiToGweiNumber(absRebalanceRequirement),
        RebalanceDirection.STAKE,
      );
      expect(metricsUpdater.setReportedRebalanceRequirement).toHaveBeenCalledWith(
        STAKING_VAULT_ADDRESS,
        weiToGweiNumber(absRebalanceRequirement),
        RebalanceDirection.STAKE,
      );
    });

    it("handles zero totalSystemBalance without division by zero", async () => {
      // Arrange
      const totalSystemBalance = 0n;
      const effectiveTarget = 0n;
      const dashboardTotalValue = 0n;
      const userFunds = 0n;
      const peekedYieldAmount = 0n;
      const peekedOutstandingNegativeYield = 0n;
      const rebalanceToleranceAmountWei = 10_000n;
      const l1MessageServiceBalance = 0n;
      const { client } = setupRebalanceTest(
        l1MessageServiceBalance,
        totalSystemBalance,
        effectiveTarget,
        dashboardTotalValue,
        userFunds,
        peekedYieldAmount,
        peekedOutstandingNegativeYield,
        rebalanceToleranceAmountWei,
      );

      // Act
      const result = await client.getRebalanceRequirements(YIELD_PROVIDER, L2_RECIPIENT);

      // Assert
      expect(result).toEqual({
        rebalanceDirection: RebalanceDirection.NONE,
        rebalanceAmount: 0n,
      });
    });
  });

  it("returns staking vault and dashboard addresses from yield provider data", async () => {
    // Arrange
    contractStub.read.getYieldProviderData.mockResolvedValueOnce(
      createYieldProviderData({
        primaryEntrypoint: L2_RECIPIENT,
        ossifiedEntrypoint: STAKING_VAULT_ADDRESS,
      }),
    );
    const client = createClient();

    // Act
    const stakingVaultAddress = await client.getLidoStakingVaultAddress(YIELD_PROVIDER);
    const dashboardAddress = await client.getLidoDashboardAddress(YIELD_PROVIDER);

    // Assert
    expect(stakingVaultAddress).toBe(STAKING_VAULT_ADDRESS);
    expect(dashboardAddress).toBe(L2_RECIPIENT);
  });

  it("pauses staking only when not already paused", async () => {
    // Arrange
    const txReceipt = { transactionHash: "0xpause" } as unknown as TransactionReceipt;
    const client = createClient();
    const pauseSpy = jest.spyOn(client, "pauseStaking").mockResolvedValue(txReceipt);
    contractStub.read.isStakingPaused.mockResolvedValueOnce(false).mockResolvedValueOnce(true);

    // Act
    const result1 = await client.pauseStakingIfNotAlready(YIELD_PROVIDER);
    const result2 = await client.pauseStakingIfNotAlready(YIELD_PROVIDER);

    // Assert
    expect(result1).toBe(txReceipt);
    expect(result2).toBeUndefined();
    expect(logger.info).toHaveBeenCalledWith(`Already paused staking for yieldProvider=${YIELD_PROVIDER}`);
    expect(pauseSpy).toHaveBeenCalledTimes(1);
  });

  it("unpauses staking only when currently paused", async () => {
    // Arrange
    const txReceipt = { transactionHash: "0xunpause" } as unknown as TransactionReceipt;
    const client = createClient();
    const unpauseSpy = jest.spyOn(client, "unpauseStaking").mockResolvedValue(txReceipt);
    contractStub.read.isStakingPaused.mockResolvedValueOnce(true).mockResolvedValueOnce(false);

    // Act
    const result1 = await client.unpauseStakingIfNotAlready(YIELD_PROVIDER);
    const result2 = await client.unpauseStakingIfNotAlready(YIELD_PROVIDER);

    // Assert
    expect(result1).toBe(txReceipt);
    expect(unpauseSpy).toHaveBeenCalledWith(YIELD_PROVIDER);
    expect(result2).toBeUndefined();
    expect(logger.info).toHaveBeenCalledWith(`Already resumed staking for yieldProvider=${YIELD_PROVIDER}`);
    expect(unpauseSpy).toHaveBeenCalledTimes(1);
  });

  it("computes available unstaking rebalance balance", async () => {
    // Arrange
    blockchainClient.getBalance.mockResolvedValueOnce(1_000n);
    contractStub.simulate.withdrawableValue.mockResolvedValueOnce({ result: 500n });
    const client = createClient();

    // Act
    const result = await client.getAvailableUnstakingRebalanceBalance(YIELD_PROVIDER);

    // Assert
    expect(blockchainClient.getBalance).toHaveBeenCalledWith(CONTRACT_ADDRESS);
    expect(contractStub.simulate.withdrawableValue).toHaveBeenCalledWith([YIELD_PROVIDER]);
    expect(result).toBe(1_500n);
  });

  it("adds to withdrawal reserve only when above threshold", async () => {
    // Arrange
    const client = createClient({ minWithdrawalThresholdEth: 1n });
    const addSpy = jest.spyOn(client, "safeAddToWithdrawalReserve").mockResolvedValue(undefined as any);
    const belowThresholdBalance = ONE_ETHER - 1n;
    const minThreshold = 1n * ONE_ETHER;
    jest
      .spyOn(client, "getAvailableUnstakingRebalanceBalance")
      .mockResolvedValueOnce(belowThresholdBalance)
      .mockResolvedValueOnce(ONE_ETHER + 100n);

    // Act
    await client.safeAddToWithdrawalReserveIfAboveThreshold(YIELD_PROVIDER, 5n);
    await client.safeAddToWithdrawalReserveIfAboveThreshold(YIELD_PROVIDER, 7n);

    // Assert
    expect(addSpy).not.toHaveBeenCalledWith(YIELD_PROVIDER, 5n);
    expect(addSpy).toHaveBeenCalledWith(YIELD_PROVIDER, 7n);
    expect(logger.info).toHaveBeenCalledWith(
      `safeAddToWithdrawalReserveIfAboveThreshold - skipping as availableWithdrawalBalance=${belowThresholdBalance} is below the minimum withdrawal threshold of ${minThreshold}`,
    );
  });

  it("adds the full available balance when calling safeMaxAddToWithdrawalReserve", async () => {
    // Arrange
    const client = createClient({ minWithdrawalThresholdEth: 1n });
    const addSpy = jest.spyOn(client, "safeAddToWithdrawalReserve").mockResolvedValue(undefined as any);
    const available = ONE_ETHER + 50n;
    jest.spyOn(client, "getAvailableUnstakingRebalanceBalance").mockResolvedValue(available);

    // Act
    await client.safeMaxAddToWithdrawalReserve(YIELD_PROVIDER);

    // Assert
    expect(addSpy).toHaveBeenCalledWith(YIELD_PROVIDER, available);
  });

  it("skips safeMaxAddToWithdrawalReserve when below the threshold", async () => {
    // Arrange
    const client = createClient({ minWithdrawalThresholdEth: 2n });
    const addSpy = jest.spyOn(client, "safeAddToWithdrawalReserve").mockResolvedValue(undefined as any);
    const belowThresholdBalance = 2n * ONE_ETHER - 1n;
    const minThreshold = 2n * ONE_ETHER;
    jest.spyOn(client, "getAvailableUnstakingRebalanceBalance").mockResolvedValue(belowThresholdBalance);

    // Act
    const result = await client.safeMaxAddToWithdrawalReserve(YIELD_PROVIDER);

    // Assert
    expect(result).toBeUndefined();
    expect(addSpy).not.toHaveBeenCalled();
    expect(logger.info).toHaveBeenCalledWith(
      `safeMaxAddToWithdrawalReserve - skipping as availableWithdrawalBalance=${belowThresholdBalance} is below the minimum withdrawal threshold of ${minThreshold}`,
    );
  });

  it("extracts withdrawal events from receipts emitted by the contract", () => {
    // Arrange
    const client = createClient();
    const log = { address: CONTRACT_ADDRESS, data: "0xdata", topics: ["0x01"] };
    mockedParseEventLogs.mockReturnValueOnce([
      {
        eventName: "WithdrawalReserveAugmented",
        args: { reserveIncrementAmount: 10n, yieldProvider: YIELD_PROVIDER },
        address: CONTRACT_ADDRESS,
      } as any,
    ]);

    // Act
    const event = client.getWithdrawalEventFromTxReceipt(createTransactionReceipt([log]));

    // Assert
    expect(event).toEqual({ reserveIncrementAmount: 10n, yieldProvider: YIELD_PROVIDER });
    expect(mockedParseEventLogs).toHaveBeenCalledWith({
      abi: contractStub.abi,
      eventName: "WithdrawalReserveAugmented",
      logs: [log],
    });
  });

  it("returns undefined when withdrawal events are absent or decoding fails", () => {
    // Arrange
    const client = createClient();
    mockedParseEventLogs.mockReturnValueOnce([]);

    // Act
    const event = client.getWithdrawalEventFromTxReceipt(
      createTransactionReceipt([{ address: CONTRACT_ADDRESS.toUpperCase(), data: "0x", topics: [] }]),
    );

    // Assert
    expect(event).toBeUndefined();
    expect(logger.debug).toHaveBeenCalledWith(
      "getWithdrawalEventFromTxReceipt - WithdrawalReserveAugmented event not found in receipt",
    );
    expect(mockedParseEventLogs).toHaveBeenCalledTimes(1);
  });

  it("ignores withdrawal events from other contracts", () => {
    // Arrange
    const client = createClient();
    const foreignLog = { address: "0x1234567890123456789012345678901234567890", data: "0x", topics: [] };
    mockedParseEventLogs.mockReturnValueOnce([
      {
        eventName: "WithdrawalReserveAugmented",
        args: { reserveIncrementAmount: 10n, yieldProvider: YIELD_PROVIDER },
        address: "0x1234567890123456789012345678901234567890",
      } as any,
    ]);

    // Act
    const event = client.getWithdrawalEventFromTxReceipt(createTransactionReceipt([foreignLog]));

    // Assert
    expect(event).toBeUndefined();
    expect(mockedParseEventLogs).toHaveBeenCalledTimes(1);
  });

  it("extracts yield reports from receipts emitted by the contract", () => {
    // Arrange
    const client = createClient();
    const log = { address: CONTRACT_ADDRESS, data: "0xfeed", topics: ["0x1111"] };
    mockedParseEventLogs.mockReturnValueOnce([
      {
        eventName: "NativeYieldReported",
        args: { yieldAmount: 12n, outstandingNegativeYield: 5n, yieldProvider: YIELD_PROVIDER },
        address: CONTRACT_ADDRESS,
      } as any,
    ]);

    // Act
    const report = client.getYieldReportFromTxReceipt(createTransactionReceipt([log]));

    // Assert
    expect(report).toEqual({ yieldAmount: 12n, outstandingNegativeYield: 5n, yieldProvider: YIELD_PROVIDER });
    expect(mockedParseEventLogs).toHaveBeenCalledWith({
      abi: contractStub.abi,
      eventName: "NativeYieldReported",
      logs: [log],
    });
  });

  it("returns undefined when yield report events are absent", () => {
    // Arrange
    const client = createClient();
    mockedParseEventLogs.mockReturnValueOnce([]);

    // Act
    const report = client.getYieldReportFromTxReceipt(
      createTransactionReceipt([{ address: CONTRACT_ADDRESS, data: "0x0", topics: [] }]),
    );

    // Assert
    expect(report).toBeUndefined();
    expect(logger.debug).toHaveBeenCalledWith("getYieldReportFromTxReceipt - NativeYieldReported event not found in receipt");
  });

  it("ignores yield report logs from other contracts", () => {
    // Arrange
    const client = createClient();
    const foreignLog = { address: "0x1234567890123456789012345678901234567890", data: "0x", topics: [] };
    mockedParseEventLogs.mockReturnValueOnce([
      {
        eventName: "NativeYieldReported",
        args: { yieldAmount: 12n, outstandingNegativeYield: 5n, yieldProvider: YIELD_PROVIDER },
        address: "0x1234567890123456789012345678901234567890",
      } as any,
    ]);

    // Act
    const report = client.getYieldReportFromTxReceipt(createTransactionReceipt([foreignLog]));

    // Assert
    expect(report).toBeUndefined();
    expect(mockedParseEventLogs).toHaveBeenCalledTimes(1);
  });
});
