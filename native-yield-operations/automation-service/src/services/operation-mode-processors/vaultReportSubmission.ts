import { Address, TransactionReceipt } from "viem";
import { ILogger, attempt } from "@consensys/linea-shared-utils";
import { ILidoAccountingReportClient } from "../../core/clients/ILidoAccountingReportClient.js";
import { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import { IVaultHub } from "../../core/clients/contracts/IVaultHub.js";

/**
 * Submits the latest vault report if enabled and the report is not fresh.
 * Checks if the vault is connected first - if it is not, skips submission.
 * Then checks if the report is fresh - if it is, skips submission to avoid unnecessary transactions.
 * If the freshness check fails, proceeds with submission as a fail-safe measure.
 *
 * @param {ILogger} logger - Logger instance for logging operations.
 * @param {IVaultHub<TransactionReceipt>} vaultHubContractClient - Client for interacting with VaultHub contracts.
 * @param {ILidoAccountingReportClient} lidoAccountingReportClient - Client for submitting Lido accounting reports.
 * @param {INativeYieldAutomationMetricsUpdater} metricsUpdater - Service for updating metrics.
 * @param {Address} vault - The vault address to submit the report for.
 * @param {boolean} shouldSubmitVaultReport - Whether to submit the vault accounting report. Can be set to false if other actors are expected to submit.
 * @param {string} logPrefix - Prefix for log messages to identify the calling context.
 * @returns {Promise<void>} A promise that resolves when the submission attempt completes (regardless of success/failure).
 */
export async function submitVaultReportIfNotFresh(
  logger: ILogger,
  vaultHubContractClient: IVaultHub<TransactionReceipt>,
  lidoAccountingReportClient: ILidoAccountingReportClient,
  metricsUpdater: INativeYieldAutomationMetricsUpdater,
  vault: Address,
  shouldSubmitVaultReport: boolean,
  logPrefix: string,
): Promise<void> {
  if (!shouldSubmitVaultReport) {
    logger.info(`${logPrefix} - Skipping vault report submission (SHOULD_SUBMIT_VAULT_REPORT=false)`);
    return;
  }

  // Check if vault is connected before attempting submission
  try {
    const isConnected = await vaultHubContractClient.isVaultConnected(vault);
    if (!isConnected) {
      logger.info(`${logPrefix} - Skipping vault report submission (vault is not connected)`);
      return;
    }
  } catch (error) {
    // Fail-safe: if connection check fails, log error but continue with submission attempt
    logger.warn(`${logPrefix} - Failed to check if vault is connected, proceeding with submission attempt: ${error}`);
  }

  // Check if report is fresh before attempting submission
  try {
    const isFresh = await vaultHubContractClient.isReportFresh(vault);
    if (isFresh) {
      logger.info(`${logPrefix} - Skipping vault report submission (report is fresh)`);
      return;
    }
  } catch (error) {
    // Fail-safe: if freshness check fails, log error but continue with submission attempt
    logger.warn(`${logPrefix} - Failed to check if report is fresh, proceeding with submission attempt: ${error}`);
  }

  // Report is not fresh or freshness check failed - proceed with submission
  logger.info(`${logPrefix} - Fetching latest vault report`);
  await lidoAccountingReportClient.getLatestSubmitVaultReportParams(vault);

  logger.info(`${logPrefix} - Submitting latest vault report`);
  const vaultResult = await attempt(
    logger,
    () => lidoAccountingReportClient.submitLatestVaultReport(vault),
    `${logPrefix} - submitLatestVaultReport failed`,
  );

  if (vaultResult.isOk()) {
    logger.info(`${logPrefix} - Vault report submission succeeded`);
    metricsUpdater.incrementLidoVaultAccountingReport(vault);
  }
}
