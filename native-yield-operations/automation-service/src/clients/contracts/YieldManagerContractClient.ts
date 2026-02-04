import { absDiff, IBlockchainClient, ILogger, weiToGweiNumber } from "@consensys/linea-shared-utils";
import {
  Address,
  concat,
  encodeAbiParameters,
  encodeFunctionData,
  getContract,
  GetContractReturnType,
  Hex,
  parseEventLogs,
  PublicClient,
  TransactionReceipt,
} from "viem";
import {
  LidoStakingVaultWithdrawalParams,
  WithdrawalRequests,
} from "../../core/entities/LidoStakingVaultWithdrawalParams.js";
import { RebalanceRequirement, RebalanceDirection } from "../../core/entities/RebalanceRequirement.js";

import { YieldManagerABI } from "../../core/abis/YieldManager.js";
import { IYieldManager, YieldProviderData } from "../../core/clients/contracts/IYieldManager.js";
import { ONE_ETHER } from "@consensys/linea-shared-utils";
import { YieldReport } from "../../core/entities/YieldReport.js";
import { StakingVaultABI } from "../../core/abis/StakingVault.js";
import { WithdrawalEvent } from "../../core/entities/WithdrawalEvent.js";
import { DashboardContractClient } from "./DashboardContractClient.js";
import { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import { IRebalanceQuotaService } from "../../core/services/IRebalanceQuotaService.js";
import { DashboardErrorsABI } from "../../core/abis/errors/DashboardErrors.js";
import { LidoStVaultYieldProviderErrorsABI } from "../../core/abis/errors/LidoStVaultYieldProviderErrors.js";
import { StakingVaultErrorsABI } from "../../core/abis/errors/StakingVaultErrors.js";
import { VaultHubErrorsABI } from "../../core/abis/errors/VaultHubErrors.js";

const YieldManagerCombinedABI = [
  ...YieldManagerABI,
  ...DashboardErrorsABI,
  ...LidoStVaultYieldProviderErrorsABI,
  ...StakingVaultErrorsABI,
  ...VaultHubErrorsABI,
] as const;

/**
 * Client for interacting with YieldManager smart contracts.
 * Provides comprehensive methods for managing yield providers, staking operations, withdrawals,
 * rebalancing, ossification, and extracting event data from transaction receipts.
 */
export class YieldManagerContractClient implements IYieldManager<TransactionReceipt> {
  private readonly contract: GetContractReturnType<typeof YieldManagerCombinedABI, PublicClient, Address>;

  /**
   * Creates a new YieldManagerContractClient instance.
   *
   * @param {ILogger} logger - Logger instance for logging operations.
   * @param {IBlockchainClient<PublicClient, TransactionReceipt>} contractClientLibrary - Blockchain client for sending transactions and reading contract data.
   * @param {Address} contractAddress - The address of the YieldManager contract.
   * @param {bigint} rebalanceToleranceAmountWei - Rebalance tolerance amount in wei (absolute tolerance band for determining when rebalancing is required).
   * @param {bigint} minWithdrawalThresholdEth - Minimum withdrawal threshold in ETH (for threshold-based withdrawal operations).
   * @param {IRebalanceQuotaService} [rebalanceQuotaService] - Service for enforcing rebalance quota limits on STAKE direction rebalances.
   * @param {INativeYieldAutomationMetricsUpdater} [metricsUpdater] - Optional metrics updater for tracking circuit breaker trips and rebalance requirements.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly contractClientLibrary: IBlockchainClient<PublicClient, TransactionReceipt>,
    private readonly contractAddress: Address,
    private readonly rebalanceToleranceAmountWei: bigint,
    private readonly minWithdrawalThresholdEth: bigint,
    private readonly rebalanceQuotaService: IRebalanceQuotaService,
    private readonly metricsUpdater?: INativeYieldAutomationMetricsUpdater,
  ) {
    this.contract = getContract({
      abi: YieldManagerCombinedABI,
      address: contractAddress,
      client: this.contractClientLibrary.getBlockchainClient(),
    });
  }

  /**
   * Gets the address of the YieldManager contract.
   *
   * @returns {Address} The contract address.
   */
  getAddress(): Address {
    return this.contractAddress;
  }

  /**
   * Gets the viem contract instance.
   *
   * @returns {GetContractReturnType} The contract instance.
   */
  getContract(): GetContractReturnType {
    return this.contract;
  }

  /**
   * Gets the balance of the YieldManager contract.
   *
   * @returns {Promise<bigint>} The contract balance in wei.
   */
  async getBalance(): Promise<bigint> {
    return this.contractClientLibrary.getBalance(this.contractAddress);
  }

  /**
   * Gets the L1 Message Service address from the YieldManager contract.
   *
   * @returns {Promise<Address>} The L1 Message Service address.
   */
  async L1_MESSAGE_SERVICE(): Promise<Address> {
    return this.contract.read.L1_MESSAGE_SERVICE();
  }

  /**
   * Gets the total system balance from the YieldManager contract.
   *
   * @returns {Promise<bigint>} The total system balance in wei.
   */
  async getTotalSystemBalance(): Promise<bigint> {
    return this.contract.read.getTotalSystemBalance();
  }

  /**
   * Gets the effective target withdrawal reserve from the YieldManager contract.
   *
   * @returns {Promise<bigint>} The effective target withdrawal reserve in wei.
   */
  async getEffectiveTargetWithdrawalReserve(): Promise<bigint> {
    return this.contract.read.getEffectiveTargetWithdrawalReserve();
  }

  /**
   * Gets the target reserve deficit from the YieldManager contract.
   *
   * @returns {Promise<bigint>} The target reserve deficit in wei.
   */
  async getTargetReserveDeficit(): Promise<bigint> {
    return this.contract.read.getTargetReserveDeficit();
  }

  /**
   * Checks if staking is paused for a yield provider.
   *
   * @param {Address} yieldProvider - The yield provider address to check.
   * @returns {Promise<boolean>} True if staking is paused, false otherwise.
   */
  async isStakingPaused(yieldProvider: Address): Promise<boolean> {
    return this.contract.read.isStakingPaused([yieldProvider]);
  }

  /**
   * Checks if ossification has been initiated for a yield provider.
   *
   * @param {Address} yieldProvider - The yield provider address to check.
   * @returns {Promise<boolean>} True if ossification is initiated, false otherwise.
   */
  async isOssificationInitiated(yieldProvider: Address): Promise<boolean> {
    return this.contract.read.isOssificationInitiated([yieldProvider]);
  }

  /**
   * Checks if a yield provider is ossified.
   *
   * @param {Address} yieldProvider - The yield provider address to check.
   * @returns {Promise<boolean>} True if the yield provider is ossified, false otherwise.
   */
  async isOssified(yieldProvider: Address): Promise<boolean> {
    return this.contract.read.isOssified([yieldProvider]);
  }

  /**
   * Gets the user funds for a yield provider from the YieldManager contract.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<bigint>} The user funds in wei.
   */
  async userFunds(yieldProvider: Address): Promise<bigint> {
    return this.contract.read.userFunds([yieldProvider]);
  }

  /**
   * Gets the withdrawable value for a yield provider using simulation.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<bigint>} The withdrawable value in wei.
   */
  async withdrawableValue(yieldProvider: Address): Promise<bigint> {
    const { result } = await this.contract.simulate.withdrawableValue([yieldProvider]);
    return result;
  }

  /**
   * Gets yield provider data from the YieldManager contract.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<YieldProviderData>} The yield provider data including entrypoints and configuration.
   */
  async getYieldProviderData(yieldProvider: Address): Promise<YieldProviderData> {
    return this.contract.read.getYieldProviderData([yieldProvider]);
  }

  /**
   * Funds a yield provider by sending a transaction to the YieldManager contract.
   *
   * @param {Address} yieldProvider - The yield provider address to fund.
   * @param {bigint} amount - The amount to fund in wei.
   * @returns {Promise<TransactionReceipt>} The transaction receipt if successful.
   */
  async fundYieldProvider(yieldProvider: Address, amount: bigint): Promise<TransactionReceipt> {
    this.logger.info(`fundYieldProvider started, yieldProvider=${yieldProvider}, amount=${amount.toString()}`);
    const calldata = encodeFunctionData({
      abi: this.contract.abi,
      functionName: "fundYieldProvider",
      args: [yieldProvider, amount],
    });

    const txReceipt = await this.contractClientLibrary.sendSignedTransaction(
      this.contractAddress,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
    this.logger.info(
      `fundYieldProvider succeeded, yieldProvider=${yieldProvider}, amount=${amount.toString()}, txHash=${txReceipt.transactionHash}`,
    );
    return txReceipt;
  }

  /**
   * Reports yield for a yield provider by sending a transaction to the YieldManager contract.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {Address} l2YieldRecipient - The L2 yield recipient address.
   * @returns {Promise<TransactionReceipt>} The transaction receipt if successful.
   */
  async reportYield(yieldProvider: Address, l2YieldRecipient: Address): Promise<TransactionReceipt> {
    this.logger.info(`reportYield started, yieldProvider=${yieldProvider}, l2YieldRecipient=${l2YieldRecipient}`);
    const calldata = encodeFunctionData({
      abi: this.contract.abi,
      functionName: "reportYield",
      args: [yieldProvider, l2YieldRecipient],
    });

    const txReceipt = await this.contractClientLibrary.sendSignedTransaction(
      this.contractAddress,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
    this.logger.info(
      `reportYield succeeded, yieldProvider=${yieldProvider}, l2YieldRecipient=${l2YieldRecipient}, txHash=${txReceipt.transactionHash}`,
    );
    return txReceipt;
  }

  /**
   * Unstakes funds from a yield provider by submitting withdrawal requests.
   * Encodes Lido withdrawal parameters, computes validator withdrawal fees, and sends a signed transaction.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {WithdrawalRequests} withdrawalParams - The withdrawal parameters including validator pubkeys and amounts.
   * @returns {Promise<TransactionReceipt>} The transaction receipt if successful.
   */
  async unstake(yieldProvider: Address, withdrawalParams: WithdrawalRequests): Promise<TransactionReceipt> {
    this.logger.info(
      `unstake started, yieldProvider=${yieldProvider}, validatorCount=${withdrawalParams.pubkeys.length}`,
    );
    this.logger.debug(`unstake started withdrawalParams`, { withdrawalParams });
    // Note: Empty amountsGwei array signals full withdrawals (validator exits) per contract logic.
    // The contract checks: if amountsGwei.length == 0, it triggers full withdrawals via addFullWithdrawalRequests.
    // If amountsGwei.length > 0, it triggers amount-driven withdrawals via addWithdrawalRequests.
    // Therefore, pubkeys and amountsGwei arrays may have mismatched lengths when requesting validator exits.
    const encodedWithdrawalParams = this._encodeLidoWithdrawalParams({
      ...withdrawalParams,
      refundRecipient: this.contractAddress,
    });
    const calldata = encodeFunctionData({
      abi: this.contract.abi,
      functionName: "unstake",
      args: [yieldProvider, encodedWithdrawalParams],
    });

    const validatorWithdrawalFee = await this._getValidatorWithdrawalFee(yieldProvider, withdrawalParams);
    const txReceipt = await this.contractClientLibrary.sendSignedTransaction(
      this.contractAddress,
      calldata,
      validatorWithdrawalFee,
      YieldManagerCombinedABI,
    );
    this.logger.info(
      `unstake succeeded, yieldProvider=${yieldProvider}, validatorCount=${withdrawalParams.pubkeys.length}, txHash=${txReceipt.transactionHash}`,
    );
    this.logger.debug(`unstake succeeded withdrawalParams`, { withdrawalParams });
    return txReceipt;
  }

  /**
   * Encodes Lido staking vault withdrawal parameters into ABI-encoded format.
   * Note: Empty amountsGwei array signals full withdrawals (validator exits) per contract logic.
   * The contract checks: if amountsGwei.length == 0, it triggers full withdrawals via addFullWithdrawalRequests.
   * If amountsGwei.length > 0, it triggers amount-driven withdrawals via addWithdrawalRequests.
   *
   * @param {LidoStakingVaultWithdrawalParams} params - The withdrawal parameters including pubkeys, amounts, and refund recipient.
   * @returns {Hex} The ABI-encoded withdrawal parameters.
   */
  private _encodeLidoWithdrawalParams(params: LidoStakingVaultWithdrawalParams): Hex {
    const concatenatedPubkeys = concat(params.pubkeys);
    return encodeAbiParameters(
      [
        { name: "pubkeys", type: "bytes" },
        { name: "amounts", type: "uint64[]" },
        { name: "refundRecipient", type: "address" },
      ],
      [concatenatedPubkeys, params.amountsGwei, params.refundRecipient],
    );
  }

  /**
   * Computes EIP7002 Withdrawal Fee for beacon chain unstaking.
   * Retrieves the validator withdrawal fee from the staking vault contract based on the number of validators.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {WithdrawalRequests} withdrawalParams - The withdrawal parameters containing validator pubkeys.
   * @returns {Promise<bigint>} The validator withdrawal fee in wei.
   */
  private async _getValidatorWithdrawalFee(
    yieldProvider: Address,
    withdrawalParams: WithdrawalRequests,
  ): Promise<bigint> {
    const vault = await this.getLidoStakingVaultAddress(yieldProvider);
    const validatorWithdrawalFee = await this.contractClientLibrary.getBlockchainClient().readContract({
      address: vault,
      abi: StakingVaultABI,
      functionName: "calculateValidatorWithdrawalFee",
      args: [BigInt(withdrawalParams.pubkeys.length)],
    });
    return validatorWithdrawalFee;
  }

  /**
   * Safely adds funds to the withdrawal reserve for a yield provider.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {bigint} amount - The amount to add to the withdrawal reserve in wei.
   * @returns {Promise<TransactionReceipt>} The transaction receipt if successful.
   */
  async safeAddToWithdrawalReserve(yieldProvider: Address, amount: bigint): Promise<TransactionReceipt> {
    this.logger.info(`safeAddToWithdrawalReserve started, yieldProvider=${yieldProvider}, amount=${amount.toString()}`);
    const calldata = encodeFunctionData({
      abi: this.contract.abi,
      functionName: "safeAddToWithdrawalReserve",
      args: [yieldProvider, amount],
    });

    const txReceipt = await this.contractClientLibrary.sendSignedTransaction(
      this.contractAddress,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
    this.logger.info(
      `safeAddToWithdrawalReserve succeeded, yieldProvider=${yieldProvider}, amount=${amount.toString()}, txHash=${txReceipt.transactionHash}`,
    );
    return txReceipt;
  }

  /**
   * Safely withdraws funds from a yield provider.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {bigint} amount - The amount to withdraw from the yield provider in wei.
   * @returns {Promise<TransactionReceipt>} The transaction receipt if successful.
   */
  async safeWithdrawFromYieldProvider(yieldProvider: Address, amount: bigint): Promise<TransactionReceipt> {
    this.logger.info(
      `safeWithdrawFromYieldProvider started, yieldProvider=${yieldProvider}, amount=${amount.toString()}`,
    );
    const calldata = encodeFunctionData({
      abi: this.contract.abi,
      functionName: "safeWithdrawFromYieldProvider",
      args: [yieldProvider, amount],
    });

    const txReceipt = await this.contractClientLibrary.sendSignedTransaction(
      this.contractAddress,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
    this.logger.info(
      `safeWithdrawFromYieldProvider succeeded, yieldProvider=${yieldProvider}, amount=${amount.toString()}, txHash=${txReceipt.transactionHash}`,
    );
    return txReceipt;
  }

  /**
   * Pauses staking for a yield provider.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<TransactionReceipt>} The transaction receipt if successful.
   */
  async pauseStaking(yieldProvider: Address): Promise<TransactionReceipt> {
    this.logger.info(`pauseStaking started, yieldProvider=${yieldProvider}`);
    const calldata = encodeFunctionData({
      abi: this.contract.abi,
      functionName: "pauseStaking",
      args: [yieldProvider],
    });

    const txReceipt = await this.contractClientLibrary.sendSignedTransaction(
      this.contractAddress,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
    this.logger.info(`pauseStaking succeeded, yieldProvider=${yieldProvider}, txHash=${txReceipt.transactionHash}`);
    return txReceipt;
  }

  /**
   * Unpauses staking for a yield provider.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<TransactionReceipt>} The transaction receipt if successful.
   */
  async unpauseStaking(yieldProvider: Address): Promise<TransactionReceipt> {
    this.logger.info(`unpauseStaking started, yieldProvider=${yieldProvider}`);
    const calldata = encodeFunctionData({
      abi: this.contract.abi,
      functionName: "unpauseStaking",
      args: [yieldProvider],
    });

    const txReceipt = await this.contractClientLibrary.sendSignedTransaction(
      this.contractAddress,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
    this.logger.info(`unpauseStaking succeeded, yieldProvider=${yieldProvider}, txHash=${txReceipt.transactionHash}`);
    return txReceipt;
  }

  /**
   * Progresses pending ossification for a yield provider.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<TransactionReceipt>} The transaction receipt if successful.
   */
  async progressPendingOssification(yieldProvider: Address): Promise<TransactionReceipt> {
    this.logger.info(`progressPendingOssification started, yieldProvider=${yieldProvider}`);
    const calldata = encodeFunctionData({
      abi: this.contract.abi,
      functionName: "progressPendingOssification",
      args: [yieldProvider],
    });

    const txReceipt = await this.contractClientLibrary.sendSignedTransaction(
      this.contractAddress,
      calldata,
      undefined,
      YieldManagerCombinedABI,
    );
    this.logger.info(
      `progressPendingOssification succeeded, yieldProvider=${yieldProvider}, txHash=${txReceipt.transactionHash}`,
    );
    return txReceipt;
  }

  /**
   * Gets rebalance requirements by comparing L1 Message Service balance with effective target withdrawal reserve.
   * Determines if rebalancing is needed (in deficit or in surplus) and calculates the required rebalance amount.
   * Returns NONE direction with 0 amount if rebalancing is not required.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {Address} l2YieldRecipient - The L2 yield recipient address.
   * @returns {Promise<RebalanceRequirement>} The rebalance requirement containing direction (NONE, STAKE, or UNSTAKE) and amount.
   */
  async getRebalanceRequirements(yieldProvider: Address, l2YieldRecipient: Address): Promise<RebalanceRequirement> {
    const [l1MessageServiceAddress, dashboardAddress, vaultAddress] = await Promise.all([
      this.L1_MESSAGE_SERVICE(),
      this.getLidoDashboardAddress(yieldProvider),
      this.getLidoStakingVaultAddress(yieldProvider),
    ]);
    const dashboardClient = DashboardContractClient.getOrCreate(dashboardAddress);
    const [
      l1MessageServiceBalance,
      totalSystemBalance,
      effectiveTargetWithdrawalReserve,
      dashboardTotalValue,
      peekedYieldReport,
      userFunds,
    ] = await Promise.all([
      this.contractClientLibrary.getBalance(l1MessageServiceAddress),
      this.getTotalSystemBalance(),
      this.getEffectiveTargetWithdrawalReserve(),
      dashboardClient.totalValue(),
      this.peekYieldReport(yieldProvider, l2YieldRecipient),
      this.userFunds(yieldProvider),
    ]);
    if (peekedYieldReport === undefined) {
      throw new Error("peekYieldReport returned undefined, cannot determine rebalance requirements");
    }
    const result = this._getRebalanceRequirements(
      vaultAddress,
      totalSystemBalance,
      l1MessageServiceBalance,
      effectiveTargetWithdrawalReserve,
      userFunds,
      dashboardTotalValue,
      peekedYieldReport.yieldAmount,
      peekedYieldReport.outstandingNegativeYield,
    );
    // Track the reported rebalance requirement (after applying tolerance band, circuit breaker, and rate limit)
    if (this.metricsUpdater) {
      this.metricsUpdater.setReportedRebalanceRequirement(
        vaultAddress,
        weiToGweiNumber(result.rebalanceAmount),
        result.rebalanceDirection,
      );
      // Clear the opposite direction to prevent stale metrics
      if (result.rebalanceDirection === RebalanceDirection.STAKE) {
        this.metricsUpdater.setReportedRebalanceRequirement(vaultAddress, 0, RebalanceDirection.UNSTAKE);
      } else if (result.rebalanceDirection === RebalanceDirection.UNSTAKE) {
        this.metricsUpdater.setReportedRebalanceRequirement(vaultAddress, 0, RebalanceDirection.STAKE);
      } else if (result.rebalanceDirection === RebalanceDirection.NONE) {
        // Clear both directions when direction is NONE
        this.metricsUpdater.setReportedRebalanceRequirement(vaultAddress, 0, RebalanceDirection.STAKE);
        this.metricsUpdater.setReportedRebalanceRequirement(vaultAddress, 0, RebalanceDirection.UNSTAKE);
      }
    }
    return result;
  }

  /**
   * Determines if rebalancing is required based on tolerance band calculations.
   * Calculates system obligations (Lido fees, node operator fees, and LST liability) from the yield report,
   * adjusts balances to exclude these obligations, and determines if the L1 Message Service balance
   * is within the tolerance band compared to the adjusted effective target withdrawal reserve.
   * Returns a RebalanceRequirement with direction (NONE, STAKE, or UNSTAKE) and amount.
   *
   * @param {Address} yieldProvider - The yield provider (vault) address for metrics tracking.
   * @param {bigint} totalSystemBalance - The total system balance.
   * @param {bigint} l1MessageServiceBalance - The L1 Message Service balance.
   * @param {bigint} effectiveTargetWithdrawalReserve - The effective target withdrawal reserve.
   * @param {bigint} userFunds - The user funds for the yield provider.
   * @param {bigint} dashboardTotalValue - The total value from the Dashboard contract.
   * @param {bigint} peekedYieldReportYieldAmount - The yield amount from the peeked yield report.
   * @param {bigint} peekedYieldReportOutstandingNegativeYield - The outstanding negative yield from the peeked yield report.
   * @returns {RebalanceRequirement} The rebalance requirement containing direction (NONE, STAKE, or UNSTAKE) and amount.
   */
  private _getRebalanceRequirements(
    vaultAddress: Address,
    totalSystemBalance: bigint,
    l1MessageServiceBalance: bigint,
    effectiveTargetWithdrawalReserve: bigint,
    userFunds: bigint,
    dashboardTotalValue: bigint,
    peekedYieldReportYieldAmount: bigint,
    peekedYieldReportOutstandingNegativeYield: bigint,
  ): RebalanceRequirement {
    this.logger.info("_getRebalanceRequirements - inputs", {
      totalSystemBalance,
      l1MessageServiceBalance,
      effectiveTargetWithdrawalReserve,
      userFunds,
      dashboardTotalValue,
      peekedYieldReportYieldAmount,
      peekedYieldReportOutstandingNegativeYield,
      rebalanceToleranceAmountWei: this.rebalanceToleranceAmountWei,
    });

    if (totalSystemBalance === 0n) {
      return {
        rebalanceDirection: RebalanceDirection.NONE,
        rebalanceAmount: 0n,
      };
    }

    // Get Lido fees + node operator fees + LST liability
    const systemObligations =
      peekedYieldReportYieldAmount > 0n
        ? dashboardTotalValue - peekedYieldReportYieldAmount - userFunds
        : peekedYieldReportOutstandingNegativeYield + dashboardTotalValue - userFunds;
    this.logger.info(`_getRebalanceRequirements - systemObligations=${systemObligations}`);

    // Get balances adjusted for system obligations
    const totalSystemBalanceExcludingObligations = totalSystemBalance - systemObligations;
    this.logger.info(
      `_getRebalanceRequirements - totalSystemBalanceExcludingObligations=${totalSystemBalanceExcludingObligations}`,
    );

    const effectiveTargetWithdrawalReserveExcludingObligations =
      (effectiveTargetWithdrawalReserve * totalSystemBalanceExcludingObligations) / totalSystemBalance;
    this.logger.info(
      `_getRebalanceRequirements - effectiveTargetWithdrawalReserveExcludingObligations=${effectiveTargetWithdrawalReserveExcludingObligations}`,
    );

    const toleranceBand = this.rebalanceToleranceAmountWei;
    this.logger.info(`_getRebalanceRequirements - toleranceBand=${toleranceBand}`);

    // Rebalance requirement = Reserve rebalance requirement + system obligations
    const absRebalanceRequirement =
      absDiff(l1MessageServiceBalance, effectiveTargetWithdrawalReserveExcludingObligations) + systemObligations;
    this.logger.info(`_getRebalanceRequirements - absRebalanceRequirement=${absRebalanceRequirement}`);

    // Determine the direction for metrics tracking (without considering tolerance band)
    const directionForMetrics: RebalanceDirection =
      l1MessageServiceBalance < effectiveTargetWithdrawalReserveExcludingObligations
        ? RebalanceDirection.UNSTAKE
        : RebalanceDirection.STAKE;

    // Track the original rebalance requirement for all paths (before tolerance band, circuit breaker, or rate limit)
    if (this.metricsUpdater) {
      this.metricsUpdater.setActualRebalanceRequirement(
        vaultAddress,
        weiToGweiNumber(absRebalanceRequirement),
        directionForMetrics,
      );
      // Clear the opposite direction to prevent stale metrics
      if (directionForMetrics === RebalanceDirection.STAKE) {
        this.metricsUpdater.setActualRebalanceRequirement(vaultAddress, 0, RebalanceDirection.UNSTAKE);
      } else if (directionForMetrics === RebalanceDirection.UNSTAKE) {
        this.metricsUpdater.setActualRebalanceRequirement(vaultAddress, 0, RebalanceDirection.STAKE);
      }
    }

    const absRebalanceAmountAfterQuota =
      directionForMetrics === RebalanceDirection.STAKE
        ? this.rebalanceQuotaService.getRebalanceAmountAfterQuota(
            vaultAddress,
            totalSystemBalance,
            absRebalanceRequirement,
          )
        : absRebalanceRequirement;
    this.logger.info(`_getRebalanceRequirements - absRebalanceAmountAfterQuota=${absRebalanceAmountAfterQuota}`);

    if (absRebalanceAmountAfterQuota < toleranceBand) {
      const result = {
        rebalanceDirection: RebalanceDirection.NONE,
        rebalanceAmount: 0n,
      };
      this.logger.info("_getRebalanceRequirements - result", result);
      return result;
    }

    if (l1MessageServiceBalance < effectiveTargetWithdrawalReserveExcludingObligations) {
      const result = {
        rebalanceDirection: RebalanceDirection.UNSTAKE,
        rebalanceAmount: absRebalanceAmountAfterQuota,
      };
      this.logger.info("_getRebalanceRequirements - result", result);
      return result;
    } else {
      // Cap staking amount to prevent l1MessageServiceBalance from falling below effectiveTargetWithdrawalReserve,
      // which would create a target reserve deficit which will cause YieldManager::fundYieldProvider revert
      const stakingRebalanceCeiling = l1MessageServiceBalance - effectiveTargetWithdrawalReserve;
      this.logger.info(`_getRebalanceRequirements - stakingRebalanceCeiling=${stakingRebalanceCeiling}`);

      const result = {
        rebalanceDirection: RebalanceDirection.STAKE,
        rebalanceAmount:
          stakingRebalanceCeiling > 0n && absRebalanceAmountAfterQuota > stakingRebalanceCeiling
            ? stakingRebalanceCeiling
            : absRebalanceAmountAfterQuota,
      };
      this.logger.info("_getRebalanceRequirements - result", result);
      return result;
    }
  }

  /**
   * Gets the Lido staking vault address (ossified entrypoint) for a yield provider.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<Address>} The Lido staking vault address.
   */
  async getLidoStakingVaultAddress(yieldProvider: Address): Promise<Address> {
    const yieldProviderData = await this.getYieldProviderData(yieldProvider);
    return yieldProviderData.ossifiedEntrypoint;
  }

  /**
   * Gets the Lido dashboard address (primary entrypoint) for a yield provider.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<Address>} The Lido dashboard address.
   */
  async getLidoDashboardAddress(yieldProvider: Address): Promise<Address> {
    const yieldProviderData = await this.getYieldProviderData(yieldProvider);
    return yieldProviderData.primaryEntrypoint;
  }

  /**
   * Pauses staking for a yield provider only if it's not already paused.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<TransactionReceipt | undefined>} The transaction receipt if staking was paused, undefined if already paused.
   */
  async pauseStakingIfNotAlready(yieldProvider: Address): Promise<TransactionReceipt | undefined> {
    if (!(await this.isStakingPaused(yieldProvider))) {
      const txReceipt = await this.pauseStaking(yieldProvider);
      return txReceipt;
    }
    this.logger.info(`Already paused staking for yieldProvider=${yieldProvider}`);
    return undefined;
  }

  /**
   * Unpauses staking for a yield provider only if it's currently paused.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<TransactionReceipt | undefined>} The transaction receipt if staking was unpaused, undefined if already unpaused.
   */
  async unpauseStakingIfNotAlready(yieldProvider: Address): Promise<TransactionReceipt | undefined> {
    if (!(await this.isStakingPaused(yieldProvider))) {
      this.logger.info(`Already resumed staking for yieldProvider=${yieldProvider}`);
      return undefined;
    }

    const txReceipt = await this.unpauseStaking(yieldProvider);
    return txReceipt;
  }

  /**
   * Gets the available unstaking rebalance balance for a yield provider.
   * Calculates the sum of YieldManager balance and yield provider withdrawable balance.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<bigint>} The available unstaking rebalance balance in wei.
   */
  async getAvailableUnstakingRebalanceBalance(yieldProvider: Address): Promise<bigint> {
    const [yieldManagerBalance, yieldProviderWithdrawableBalance] = await Promise.all([
      this.contractClientLibrary.getBalance(this.contractAddress),
      this.withdrawableValue(yieldProvider),
    ]);
    return yieldManagerBalance + yieldProviderWithdrawableBalance;
  }

  /**
   * Safely adds funds to withdrawal reserve only if the available withdrawal balance is above the minimum threshold.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {bigint} amount - The amount to add to the withdrawal reserve in wei.
   * @returns {Promise<TransactionReceipt | undefined>} The transaction receipt if successful, undefined if below threshold.
   */
  async safeAddToWithdrawalReserveIfAboveThreshold(
    yieldProvider: Address,
    amount: bigint,
  ): Promise<TransactionReceipt | undefined> {
    const availableWithdrawalBalance = await this.getAvailableUnstakingRebalanceBalance(yieldProvider);
    if (availableWithdrawalBalance < this.minWithdrawalThresholdEth * ONE_ETHER) {
      this.logger.info(
        `safeAddToWithdrawalReserveIfAboveThreshold - skipping as availableWithdrawalBalance=${availableWithdrawalBalance} is below the minimum withdrawal threshold of ${this.minWithdrawalThresholdEth * ONE_ETHER}`,
      );
      return undefined;
    }
    return await this.safeAddToWithdrawalReserve(yieldProvider, amount);
  }

  /**
   * Safely adds the maximum available withdrawal balance to the withdrawal reserve.
   * Only proceeds if the available withdrawal balance is above the minimum threshold.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @returns {Promise<TransactionReceipt | undefined>} The transaction receipt if successful, undefined if below threshold.
   */
  async safeMaxAddToWithdrawalReserve(yieldProvider: Address): Promise<TransactionReceipt | undefined> {
    const availableWithdrawalBalance = await this.getAvailableUnstakingRebalanceBalance(yieldProvider);
    if (availableWithdrawalBalance < this.minWithdrawalThresholdEth * ONE_ETHER) {
      this.logger.info(
        `safeMaxAddToWithdrawalReserve - skipping as availableWithdrawalBalance=${availableWithdrawalBalance} is below the minimum withdrawal threshold of ${this.minWithdrawalThresholdEth * ONE_ETHER}`,
      );
      return undefined;
    }
    return await this.safeAddToWithdrawalReserve(yieldProvider, availableWithdrawalBalance);
  }

  /**
   * Extracts withdrawal event data from a transaction receipt by decoding WithdrawalReserveAugmented events.
   * Only decodes logs emitted by this contract. Skips unrelated logs (from the same contract or different ABIs).
   * If event not found, returns undefined.
   *
   * @param {TransactionReceipt} txReceipt - The transaction receipt to search for WithdrawalReserveAugmented events.
   * @returns {WithdrawalEvent | undefined} The withdrawal event containing reserveIncrementAmount and yieldProvider, or undefined if not found.
   */
  getWithdrawalEventFromTxReceipt(txReceipt: TransactionReceipt): WithdrawalEvent | undefined {
    const logs = parseEventLogs({
      abi: this.contract.abi,
      eventName: "WithdrawalReserveAugmented",
      logs: txReceipt.logs,
    });

    const event = logs.find((log) => log.address.toLowerCase() === this.contractAddress.toLowerCase());
    if (!event) {
      this.logger.debug("getWithdrawalEventFromTxReceipt - WithdrawalReserveAugmented event not found in receipt");
      return undefined;
    }

    const { reserveIncrementAmount, yieldProvider } = event.args;
    return { reserveIncrementAmount, yieldProvider };
  }

  /**
   * Extracts yield report data from a transaction receipt by decoding NativeYieldReported events.
   * Only decodes logs emitted by this contract. Skips unrelated logs (from the same contract or different ABIs).
   * If event not found, returns undefined.
   *
   * @param {TransactionReceipt} txReceipt - The transaction receipt to search for NativeYieldReported events.
   * @returns {YieldReport | undefined} The yield report containing yieldAmount, outstandingNegativeYield, and yieldProvider, or undefined if not found.
   */
  getYieldReportFromTxReceipt(txReceipt: TransactionReceipt): YieldReport | undefined {
    const logs = parseEventLogs({
      abi: this.contract.abi,
      eventName: "NativeYieldReported",
      logs: txReceipt.logs,
    });

    const event = logs.find((log) => log.address.toLowerCase() === this.contractAddress.toLowerCase());
    if (!event) {
      this.logger.debug("getYieldReportFromTxReceipt - NativeYieldReported event not found in receipt");
      return undefined;
    }

    const { yieldAmount, outstandingNegativeYield, yieldProvider } = event.args;
    return {
      yieldAmount,
      outstandingNegativeYield,
      yieldProvider,
    };
  }

  /**
   * Simulates the reportYield function call to peek at the yield report without executing a transaction.
   * Returns the yield report data that would be generated if reportYield were called.
   *
   * @param {Address} yieldProvider - The yield provider address.
   * @param {Address} l2YieldRecipient - The L2 yield recipient address.
   * @returns {Promise<YieldReport | undefined>} The yield report containing yieldAmount, outstandingNegativeYield, and yieldProvider, or undefined if simulation fails.
   */
  async peekYieldReport(yieldProvider: Address, l2YieldRecipient: Address): Promise<YieldReport | undefined> {
    try {
      const { result } = await this.contract.simulate.reportYield([yieldProvider, l2YieldRecipient], {
        account: this.contractClientLibrary.getSignerAddress(),
      });
      const [newReportedYield, outstandingNegativeYield] = result;
      return {
        yieldAmount: newReportedYield,
        outstandingNegativeYield,
        yieldProvider,
      };
    } catch (error) {
      this.logger.error(
        `peekYieldReport failed, yieldProvider=${yieldProvider}, l2YieldRecipient=${l2YieldRecipient}`,
        { error },
      );
      return undefined;
    }
  }
}
