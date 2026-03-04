import {
  ILogger,
  weiToGweiNumber,
  IBeaconNodeAPIClient,
  PendingPartialWithdrawal,
  PendingDeposit,
  get0x02WithdrawalCredentials,
  wait,
} from "@consensys/linea-shared-utils";
import { IValidatorDataClient } from "../core/clients/IValidatorDataClient.js";
import { INativeYieldAutomationMetricsUpdater } from "../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import { IYieldManager, YieldProviderData } from "../core/clients/contracts/IYieldManager.js";
import { IVaultHub } from "../core/clients/contracts/IVaultHub.js";
import { ISTETH } from "../core/clients/contracts/ISTETH.js";
import { ExitedValidator, ExitingValidator, ValidatorBalance } from "../core/entities/Validator.js";
import { Address, Hex, TransactionReceipt } from "viem";
import { IGaugeMetricsPoller } from "../core/services/IGaugeMetricsPoller.js";
import { IOperationLoop } from "./IOperationLoop.js";
import { DashboardContractClient } from "../clients/contracts/DashboardContractClient.js";

/**
 * Polls various data sources and updates gauge metrics.
 * Handles updating metrics like total pending partial withdrawals and cumulative yield reported.
 * Runs in a continuous loop at a configurable interval.
 */
export class GaugeMetricsPoller implements IGaugeMetricsPoller, IOperationLoop {
  private isRunning = false;

  /**
   * Creates a new GaugeMetricsPoller instance.
   *
   * @param {ILogger} logger - Logger instance for logging errors.
   * @param {IValidatorDataClient} validatorDataClient - Client for retrieving validator data.
   * @param {INativeYieldAutomationMetricsUpdater} metricsUpdater - Service for updating metrics.
   * @param {IYieldManager<TransactionReceipt>} yieldManagerContractClient - Client for reading yield provider data from YieldManager contract.
   * @param {IVaultHub<TransactionReceipt>} vaultHubContractClient - Client for reading vault data from VaultHub contract.
   * @param {Address} yieldProvider - The yield provider address.
   * @param {IBeaconNodeAPIClient} beaconNodeApiClient - Client for retrieving pending partial withdrawals from beacon chain.
   * @param {number} pollIntervalMs - Polling interval in milliseconds between gauge metrics updates.
   * @param {ISTETH} stethContractClient - Client for reading STETH contract data.
   * @param {IBeaconNodeAPIClient} [referenceBeaconNodeApiClient] - Optional reference beacon client for epoch drift detection.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly validatorDataClient: IValidatorDataClient,
    private readonly metricsUpdater: INativeYieldAutomationMetricsUpdater,
    private readonly yieldManagerContractClient: IYieldManager<TransactionReceipt>,
    private readonly vaultHubContractClient: IVaultHub<TransactionReceipt>,
    private readonly yieldProvider: Address,
    private readonly beaconNodeApiClient: IBeaconNodeAPIClient,
    private readonly pollIntervalMs: number,
    private readonly stethContractClient: ISTETH,
    private readonly referenceBeaconNodeApiClient?: IBeaconNodeAPIClient,
  ) {}

  /**
   * Polls data sources and updates gauge metrics.
   * Updates both validator-related metrics and contract-based metrics in parallel.
   * Gracefully handles failures - if one metric update fails, others will still proceed.
   * Errors are logged but do not propagate to prevent breaking the polling loop.
   *
   * @returns {Promise<void>} A promise that resolves when gauge metrics are updated.
   */
  async poll(): Promise<void> {
    // Fetch validator data, pending withdrawals, pending deposits, vault address, and yield provider data in parallel
    // Use Promise.allSettled to handle failures gracefully
    const fetchResults = await Promise.allSettled([
      this.validatorDataClient.getActiveValidators(),
      this.validatorDataClient.getExitingValidators(),
      this.validatorDataClient.getExitedValidators(),
      this.beaconNodeApiClient.getPendingPartialWithdrawals(),
      this.beaconNodeApiClient.getPendingDeposits(),
      this.yieldManagerContractClient.getLidoStakingVaultAddress(this.yieldProvider),
      this.yieldManagerContractClient.getYieldProviderData(this.yieldProvider),
    ]);

    const allValidators = fetchResults[0].status === "fulfilled" ? fetchResults[0].value : undefined;
    const exitingValidators = fetchResults[1].status === "fulfilled" ? fetchResults[1].value : undefined;
    const exitedValidators = fetchResults[2].status === "fulfilled" ? fetchResults[2].value : undefined;
    const pendingWithdrawalsQueue = fetchResults[3].status === "fulfilled" ? fetchResults[3].value : undefined;
    const pendingDeposits = fetchResults[4].status === "fulfilled" ? fetchResults[4].value : undefined;
    const vault = fetchResults[5].status === "fulfilled" ? fetchResults[5].value : undefined;
    const yieldProviderData = fetchResults[6].status === "fulfilled" ? fetchResults[6].value : undefined;

    // Log fetch failures if any
    if (fetchResults[0].status === "rejected") {
      this.logger.error("Failed to fetch active validators", { error: fetchResults[0].reason });
    }
    if (fetchResults[1].status === "rejected") {
      this.logger.error("Failed to fetch exiting validators", { error: fetchResults[1].reason });
    }
    if (fetchResults[2].status === "rejected") {
      this.logger.error("Failed to fetch exited validators", { error: fetchResults[2].reason });
    }
    if (fetchResults[3].status === "rejected") {
      this.logger.error("Failed to fetch pending partial withdrawals", { error: fetchResults[3].reason });
    }
    if (fetchResults[4].status === "rejected") {
      this.logger.error("Failed to fetch pending deposits", { error: fetchResults[4].reason });
    }
    if (fetchResults[5].status === "rejected") {
      this.logger.error("Failed to fetch vault address, skipping vault-dependent metrics", {
        error: fetchResults[5].reason,
      });
    }
    if (fetchResults[6].status === "rejected") {
      this.logger.error("Failed to fetch yield provider data", { error: fetchResults[6].reason });
    }

    // Beacon chain epoch drift detection (only if reference client is configured)
    if (this.referenceBeaconNodeApiClient !== undefined) {
      const epochResults = await Promise.allSettled([
        this.beaconNodeApiClient.getCurrentEpoch(),
        this.referenceBeaconNodeApiClient.getCurrentEpoch(),
      ]);

      const primaryEpoch = epochResults[0].status === "fulfilled" ? epochResults[0].value : undefined;
      const referenceEpoch = epochResults[1].status === "fulfilled" ? epochResults[1].value : undefined;

      if (epochResults[0].status === "rejected") {
        this.logger.warn("Failed to fetch primary beacon epoch for drift check", { error: epochResults[0].reason });
      }
      if (epochResults[1].status === "rejected") {
        this.logger.warn("Failed to fetch reference beacon epoch for drift check", { error: epochResults[1].reason });
      }

      this._updateBeaconChainEpochDriftGauge(primaryEpoch, referenceEpoch);
    }

    // Update metrics in parallel for efficiency
    // Use Promise.allSettled to ensure all updates are attempted even if one fails
    const updatePromises: Promise<void>[] = [
      this._updateTotalPendingPartialWithdrawalsGauge(allValidators, pendingWithdrawalsQueue),
      this._updatePendingPartialWithdrawalsQueueGauge(allValidators, pendingWithdrawalsQueue),
      this._updateTotalValidatorBalanceGauge(allValidators),
      this._updateValidatorStakedAmountGwei(allValidators),
      this._updatePendingExitQueueAmountGwei(exitingValidators),
      this._updateLastTotalPendingExitGwei(exitingValidators),
      this._updatePendingFullWithdrawalQueueAmountGwei(exitedValidators),
      this._updateLastTotalPendingFullWithdrawalGwei(exitedValidators),
    ];

    // Only add vault-dependent metrics if we successfully fetched the vault address
    if (vault !== undefined) {
      updatePromises.push(this._updateLastVaultReportTimestampGauge(vault));
      updatePromises.push(this._updatePendingDepositsQueueGauge(vault, pendingDeposits));
      updatePromises.push(this._updateTotalPendingDepositsGauge(vault, pendingDeposits));
      // Only add yield provider data metrics if we successfully fetched the data
      if (yieldProviderData !== undefined) {
        updatePromises.push(
          this._updateYieldReportedCumulativeGauge(vault, yieldProviderData),
          this._updateLstLiabilityPrincipalGauge(vault, yieldProviderData),
          this._updateLastReportedNegativeYieldGauge(vault, yieldProviderData),
          this._updateLidoLstLiabilityGauge(vault, yieldProviderData),
        );
      }
    }

    const results = await Promise.allSettled(updatePromises);

    // Log any failures
    results.forEach((result, index) => {
      if (result.status === "rejected") {
        const metricNames = [
          "total pending partial withdrawals",
          "pending partial withdrawals queue",
          "total validator balance",
          "validator staked amount",
          "pending exit queue",
          "total pending exit",
          "pending full withdrawal queue",
          "total pending full withdrawal",
          "last vault report timestamp",
          "pending deposits queue",
          "total pending deposits",
          "yield reported cumulative",
          "lst liability principal",
          "last reported negative yield",
          "lido lst liability",
        ];
        const metricName = metricNames[index] || "unknown";
        this.logger.error(`Failed to update ${metricName} gauge metric`, { error: result.reason });
      }
    });
  }

  /**
   * Starts the gauge metrics polling loop.
   * Sets the running flag and begins polling at the configured interval.
   * If already running, returns immediately without starting a new loop.
   *
   * @returns {Promise<void>} A promise that resolves when the loop starts (but does not resolve until the loop stops).
   */
  public async start(): Promise<void> {
    if (this.isRunning) {
      this.logger.debug("GaugeMetricsPoller.start() - already running, skipping");
      return;
    }

    this.isRunning = true;
    this.logger.info(`Starting gauge metrics polling loop`);
    await this.pollLoop();
  }

  /**
   * Stops the gauge metrics polling loop.
   * Sets the running flag to false, which causes the loop to exit on its next iteration.
   * If not running, returns immediately.
   */
  public stop(): void {
    if (!this.isRunning) {
      this.logger.debug("GaugeMetricsPoller.stop() - not running, skipping");
      return;
    }

    this.isRunning = false;
    this.logger.info(`Stopped gauge metrics polling loop`);
  }

  /**
   * Main loop that continuously polls and updates gauge metrics at the configured interval.
   * Errors in poll() are handled internally and do not break the loop.
   *
   * @returns {Promise<void>} A promise that resolves when the loop exits (when isRunning becomes false).
   */
  private async pollLoop(): Promise<void> {
    while (this.isRunning) {
      await this.poll();
      if (this.isRunning) {
        await wait(this.pollIntervalMs);
      }
    }
  }

  /**
   * Updates the total pending partial withdrawals gauge metric.
   * Follows the same pattern as BeaconChainStakingClient.submitWithdrawalRequestsToFulfilAmount.
   *
   * @param {ValidatorBalance[] | undefined} allValidators - Array of active validators, or undefined.
   * @param {PendingPartialWithdrawal[] | undefined} pendingWithdrawalsQueue - Array of pending partial withdrawals, or undefined.
   * @returns {Promise<void>} A promise that resolves when the gauge is updated (or returns early with a warning if validator data is unavailable).
   */
  private async _updateTotalPendingPartialWithdrawalsGauge(
    allValidators: ValidatorBalance[] | undefined,
    pendingWithdrawalsQueue: PendingPartialWithdrawal[] | undefined,
  ): Promise<void> {
    const joinedValidatorList = this.validatorDataClient.joinValidatorsWithPendingWithdrawals(
      allValidators,
      pendingWithdrawalsQueue,
    );
    if (joinedValidatorList === undefined) {
      this.logger.warn("Skipping total pending partial withdrawals gauge update: validator data unavailable");
      return;
    }
    const totalPendingPartialWithdrawalsWei =
      this.validatorDataClient.getTotalPendingPartialWithdrawalsWei(joinedValidatorList);
    this.metricsUpdater.setLastTotalPendingPartialWithdrawalsGwei(weiToGweiNumber(totalPendingPartialWithdrawalsWei));
  }

  /**
   * Updates the per-validator pending partial withdrawal queue gauge metrics.
   * Filters and aggregates pending withdrawals by validator_index and withdrawable_epoch,
   * then updates metrics for each aggregated withdrawal.
   *
   * @param {ValidatorBalance[] | undefined} allValidators - Array of active validators, or undefined.
   * @param {PendingPartialWithdrawal[] | undefined} pendingWithdrawalsQueue - Array of pending partial withdrawals, or undefined.
   * @returns {Promise<void>} A promise that resolves when the gauges are updated (or returns early with a warning if validator data is unavailable).
   */
  private async _updatePendingPartialWithdrawalsQueueGauge(
    allValidators: ValidatorBalance[] | undefined,
    pendingWithdrawalsQueue: PendingPartialWithdrawal[] | undefined,
  ): Promise<void> {
    const aggregatedWithdrawals = this.validatorDataClient.getFilteredAndAggregatedPendingWithdrawals(
      allValidators,
      pendingWithdrawalsQueue,
    );
    if (aggregatedWithdrawals === undefined) {
      this.logger.warn("Skipping pending partial withdrawals queue gauge update: aggregated withdrawals unavailable");
      return;
    }
    for (const withdrawal of aggregatedWithdrawals) {
      const amountGwei = Number(withdrawal.amount);
      this.metricsUpdater.setPendingPartialWithdrawalQueueAmountGwei(
        withdrawal.pubkey as Hex,
        withdrawal.withdrawable_epoch,
        amountGwei,
      );
    }
  }

  /**
   * Updates the total validator balance gauge metric.
   *
   * @param {ValidatorBalance[] | undefined} allValidators - Array of active validators, or undefined.
   * @returns {Promise<void>} A promise that resolves when the gauge is updated (or returns early with a warning if validator data is unavailable).
   */
  private async _updateTotalValidatorBalanceGauge(allValidators: ValidatorBalance[] | undefined): Promise<void> {
    const totalValidatorBalanceGwei = this.validatorDataClient.getTotalValidatorBalanceGwei(allValidators);
    if (totalValidatorBalanceGwei === undefined) {
      this.logger.warn("Skipping total validator balance gauge update: validator balance unavailable");
      return;
    }
    this.metricsUpdater.setLastTotalValidatorBalanceGwei(Number(totalValidatorBalanceGwei));
  }

  /**
   * Updates the per-validator staked amount gauge metrics.
   * Iterates through active validators and updates metrics for each validator.
   *
   * @param {ValidatorBalance[] | undefined} allValidators - Array of active validators, or undefined.
   * @returns {Promise<void>} A promise that resolves when the gauges are updated (or returns early with a warning if validator data is unavailable).
   */
  private async _updateValidatorStakedAmountGwei(allValidators: ValidatorBalance[] | undefined): Promise<void> {
    if (allValidators === undefined || allValidators.length === 0) {
      this.logger.warn("Skipping validator staked amount gauge update: active validators data unavailable or empty");
      return;
    }
    for (const validator of allValidators) {
      const amountGwei = Number(validator.balance);
      this.metricsUpdater.setValidatorStakedAmountGwei(validator.publicKey as Hex, amountGwei);
    }
  }

  /**
   * Updates the cumulative yield reported gauge metric from the YieldManager contract.
   *
   * @param {Address} vault - The vault address to use for the metric.
   * @param {YieldProviderData} yieldProviderData - The yield provider data from the YieldManager contract.
   * @returns {Promise<void>} A promise that resolves when the gauge is updated.
   */
  private async _updateYieldReportedCumulativeGauge(
    vault: Address,
    yieldProviderData: YieldProviderData,
  ): Promise<void> {
    this.metricsUpdater.setYieldReportedCumulative(vault, weiToGweiNumber(yieldProviderData.yieldReportedCumulative));
  }

  /**
   * Updates the LST liability principal gauge metric from the YieldManager contract.
   *
   * @param {Address} vault - The vault address to use for the metric.
   * @param {YieldProviderData} yieldProviderData - The yield provider data from the YieldManager contract.
   * @returns {Promise<void>} A promise that resolves when the gauge is updated.
   */
  private async _updateLstLiabilityPrincipalGauge(vault: Address, yieldProviderData: YieldProviderData): Promise<void> {
    this.metricsUpdater.setLstLiabilityPrincipalGwei(vault, weiToGweiNumber(yieldProviderData.lstLiabilityPrincipal));
  }

  /**
   * Updates the last reported negative yield gauge metric from the YieldManager contract.
   *
   * @param {Address} vault - The vault address to use for the metric.
   * @param {YieldProviderData} yieldProviderData - The yield provider data from the YieldManager contract.
   * @returns {Promise<void>} A promise that resolves when the gauge is updated.
   */
  private async _updateLastReportedNegativeYieldGauge(
    vault: Address,
    yieldProviderData: YieldProviderData,
  ): Promise<void> {
    this.metricsUpdater.setLastReportedNegativeYield(
      vault,
      weiToGweiNumber(yieldProviderData.lastReportedNegativeYield),
    );
  }

  /**
   * Updates the Lido LST liability gauge metric.
   * Fetches liability shares from Dashboard contract, converts to ETH using STETH contract,
   * and updates the metric in gwei.
   *
   * @param {Address} vault - The vault address to use for the metric.
   * @param {YieldProviderData} yieldProviderData - The yield provider data containing dashboard address.
   * @returns {Promise<void>} A promise that resolves when the gauge is updated (or returns early on error).
   */
  private async _updateLidoLstLiabilityGauge(vault: Address, yieldProviderData: YieldProviderData): Promise<void> {
    try {
      const dashboardAddress = yieldProviderData.primaryEntrypoint;
      const dashboardClient = DashboardContractClient.getOrCreate(dashboardAddress);

      const liabilityShares = await dashboardClient.liabilityShares();
      const pooledEth = await this.stethContractClient.getPooledEthBySharesRoundUp(liabilityShares);

      if (pooledEth === undefined) {
        this.logger.warn("Skipping Lido LST liability gauge update: getPooledEthBySharesRoundUp returned undefined");
        return;
      }

      const amountGwei = weiToGweiNumber(pooledEth);
      this.metricsUpdater.setLidoLstLiabilityGwei(vault, amountGwei);
    } catch (error) {
      this.logger.error("Failed to update Lido LST liability gauge", { error, vault });
    }
  }

  /**
   * Updates the last vault report timestamp gauge metric from the VaultHub contract.
   *
   * @param {Address} vault - The vault address to use for the metric.
   * @returns {Promise<void>} A promise that resolves when the gauge is updated.
   */
  private async _updateLastVaultReportTimestampGauge(vault: Address): Promise<void> {
    const timestamp = await this.vaultHubContractClient.getLatestVaultReportTimestamp(vault);
    this.metricsUpdater.setLastVaultReportTimestamp(vault, Number(timestamp));
  }

  /**
   * Updates the pending deposits queue gauge metrics.
   * Filters pending deposits by withdrawal credentials matching the vault address,
   * then updates metrics for each matching deposit.
   *
   * @param {Address} vault - The vault address to use for filtering deposits.
   * @param {PendingDeposit[] | undefined} pendingDeposits - Array of pending deposits, or undefined.
   * @returns {Promise<void>} A promise that resolves when the gauges are updated (or returns early with a warning if deposit data is unavailable).
   */
  private async _updatePendingDepositsQueueGauge(
    vault: Address,
    pendingDeposits: PendingDeposit[] | undefined,
  ): Promise<void> {
    if (pendingDeposits === undefined) {
      this.logger.warn("Skipping pending deposits queue gauge update: pending deposits data unavailable");
      return;
    }

    const vaultWithdrawalCredentials = get0x02WithdrawalCredentials(vault);
    const matchingDeposits = pendingDeposits.filter(
      (deposit) => deposit.withdrawal_credentials.toLowerCase() === vaultWithdrawalCredentials.toLowerCase(),
    );

    for (const deposit of matchingDeposits) {
      // amount is already in gwei (as number)
      this.metricsUpdater.setPendingDepositQueueAmountGwei(deposit.pubkey as Hex, deposit.slot, deposit.amount);
    }
  }

  /**
   * Updates the total pending deposits gauge metric.
   * Filters pending deposits by withdrawal credentials matching the vault address,
   * then sums and updates the total metric.
   *
   * @param {Address} vault - The vault address to use for filtering deposits.
   * @param {PendingDeposit[] | undefined} pendingDeposits - Array of pending deposits, or undefined.
   * @returns {Promise<void>} A promise that resolves when the gauge is updated (or returns early with a warning if deposit data is unavailable).
   */
  private async _updateTotalPendingDepositsGauge(
    vault: Address,
    pendingDeposits: PendingDeposit[] | undefined,
  ): Promise<void> {
    if (pendingDeposits === undefined) {
      this.logger.warn("Skipping total pending deposits gauge update: pending deposits data unavailable");
      return;
    }

    const vaultWithdrawalCredentials = get0x02WithdrawalCredentials(vault);
    const matchingDeposits = pendingDeposits.filter(
      (deposit) => deposit.withdrawal_credentials.toLowerCase() === vaultWithdrawalCredentials.toLowerCase(),
    );

    const totalAmount = matchingDeposits.reduce((sum, deposit) => sum + deposit.amount, 0);
    // amount is already in gwei (as number)
    this.metricsUpdater.setLastTotalPendingDepositGwei(totalAmount);
  }

  /**
   * Updates the per-validator pending exit queue gauge metrics.
   * Iterates through exiting validators and updates metrics for each validator.
   *
   * @param {ExitingValidator[] | undefined} exitingValidators - Array of exiting validators, or undefined.
   * @returns {Promise<void>} A promise that resolves when the gauges are updated (or returns early with a warning if validator data is unavailable).
   */
  private async _updatePendingExitQueueAmountGwei(exitingValidators: ExitingValidator[] | undefined): Promise<void> {
    if (exitingValidators === undefined || exitingValidators.length === 0) {
      this.logger.warn("Skipping pending exit queue gauge update: exiting validators data unavailable or empty");
      return;
    }
    for (const validator of exitingValidators) {
      const amountGwei = Number(validator.balance);
      this.metricsUpdater.setPendingExitQueueAmountGwei(
        validator.publicKey as Hex,
        validator.exitEpoch,
        amountGwei,
        validator.slashed,
      );
    }
  }

  /**
   * Updates the total pending exit amount gauge metric.
   *
   * @param {ExitingValidator[] | undefined} exitingValidators - Array of exiting validators, or undefined.
   * @returns {Promise<void>} A promise that resolves when the gauge is updated (or returns early with a warning if validator data is unavailable).
   */
  private async _updateLastTotalPendingExitGwei(exitingValidators: ExitingValidator[] | undefined): Promise<void> {
    const totalPendingExitGwei = this.validatorDataClient.getTotalBalanceOfExitingValidators(exitingValidators);
    if (totalPendingExitGwei === undefined) {
      this.logger.warn("Skipping total pending exit gauge update: total balance unavailable");
      return;
    }
    this.metricsUpdater.setLastTotalPendingExitGwei(Number(totalPendingExitGwei));
  }

  /**
   * Updates the per-validator pending full withdrawal queue gauge metrics.
   * Iterates through exited validators and updates metrics for each validator.
   *
   * @param {ExitedValidator[] | undefined} exitedValidators - Array of exited validators, or undefined.
   * @returns {Promise<void>} A promise that resolves when the gauges are updated (or returns early with a warning if validator data is unavailable).
   */
  private async _updatePendingFullWithdrawalQueueAmountGwei(
    exitedValidators: ExitedValidator[] | undefined,
  ): Promise<void> {
    if (exitedValidators === undefined || exitedValidators.length === 0) {
      this.logger.warn(
        "Skipping pending full withdrawal queue gauge update: exited validators data unavailable or empty",
      );
      return;
    }
    for (const validator of exitedValidators) {
      const amountGwei = Number(validator.balance);
      this.metricsUpdater.setPendingFullWithdrawalQueueAmountGwei(
        validator.publicKey as Hex,
        validator.withdrawableEpoch,
        amountGwei,
        validator.slashed,
      );
    }
  }

  /**
   * Updates the total pending full withdrawal amount gauge metric.
   *
   * @param {ExitedValidator[] | undefined} exitedValidators - Array of exited validators, or undefined.
   * @returns {Promise<void>} A promise that resolves when the gauge is updated (or returns early with a warning if validator data is unavailable).
   */
  private async _updateLastTotalPendingFullWithdrawalGwei(
    exitedValidators: ExitedValidator[] | undefined,
  ): Promise<void> {
    const totalPendingFullWithdrawalGwei = this.validatorDataClient.getTotalBalanceOfExitedValidators(exitedValidators);
    if (totalPendingFullWithdrawalGwei === undefined) {
      this.logger.warn("Skipping total pending full withdrawal gauge update: total balance unavailable");
      return;
    }
    this.metricsUpdater.setLastTotalPendingFullWithdrawalGwei(Number(totalPendingFullWithdrawalGwei));
  }

  /**
   * Updates the beacon chain epoch drift gauge metric.
   * Compares epochs from the primary and reference beacon chain RPCs.
   *
   * @param {number | undefined} primaryEpoch - The epoch from the primary beacon RPC.
   * @param {number | undefined} referenceEpoch - The epoch from the reference beacon RPC.
   */
  private _updateBeaconChainEpochDriftGauge(
    primaryEpoch: number | undefined,
    referenceEpoch: number | undefined,
  ): void {
    if (primaryEpoch === undefined || referenceEpoch === undefined) {
      this.logger.warn("Beacon chain epoch drift check failed: one or both epoch values unavailable", {
        primaryEpoch,
        referenceEpoch,
      });
      this.metricsUpdater.setBeaconChainEpochDrift(-1);
      return;
    }

    const drift = Math.abs(primaryEpoch - referenceEpoch);
    this.logger.info("Beacon chain epoch drift check complete", { primaryEpoch, referenceEpoch, drift });
    this.metricsUpdater.setBeaconChainEpochDrift(drift);
  }
}
