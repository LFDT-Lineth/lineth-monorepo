import { ILogger } from "@consensys/linea-shared-utils";
import { jest, describe, it, expect, beforeEach } from "@jest/globals";
import { ResultAsync, ok, err } from "neverthrow";
import type { Address, TransactionReceipt } from "viem";

import { createLoggerMock } from "../../../__tests__/helpers/index.js";
import type { INativeYieldAutomationMetricsUpdater } from "../../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import type { ILidoAccountingReportClient } from "../../../core/clients/ILidoAccountingReportClient.js";
import type { IVaultHub } from "../../../core/clients/contracts/IVaultHub.js";
import { submitVaultReportIfNotFresh } from "../vaultReportSubmission.js";

jest.mock("@consensys/linea-shared-utils", () => {
  const actual = jest.requireActual("@consensys/linea-shared-utils") as typeof import("@consensys/linea-shared-utils");
  return {
    ...actual,
    attempt: jest.fn(),
  };
});

import { attempt } from "@consensys/linea-shared-utils";

const attemptMock = attempt as jest.MockedFunction<typeof attempt>;

// Semantic constants
const VAULT_ADDRESS = "0x2222222222222222222222222222222222222222" as Address;
const LOG_PREFIX = "test";

describe("submitVaultReportIfNotFresh", () => {
  let logger: jest.Mocked<ILogger>;
  let vaultHubClient: jest.Mocked<IVaultHub<TransactionReceipt>>;
  let lidoReportClient: jest.Mocked<ILidoAccountingReportClient>;
  let metricsUpdater: jest.Mocked<INativeYieldAutomationMetricsUpdater>;

  const createVaultHubClientMock = (): jest.Mocked<IVaultHub<TransactionReceipt>> => ({
    isVaultConnected: jest.fn(),
    isReportFresh: jest.fn(),
  }) as unknown as jest.Mocked<IVaultHub<TransactionReceipt>>;

  const createLidoReportClientMock = (): jest.Mocked<ILidoAccountingReportClient> => ({
    getLatestSubmitVaultReportParams: jest.fn(),
    submitLatestVaultReport: jest.fn(),
  }) as unknown as jest.Mocked<ILidoAccountingReportClient>;

  const createMetricsUpdaterMock = (): jest.Mocked<INativeYieldAutomationMetricsUpdater> => ({
    incrementLidoVaultAccountingReport: jest.fn(),
  }) as unknown as jest.Mocked<INativeYieldAutomationMetricsUpdater>;

  const createVaultReportParams = () => ({
    vault: VAULT_ADDRESS,
    totalValue: 0n,
    cumulativeLidoFees: 0n,
    liabilityShares: 0n,
    maxLiabilityShares: 0n,
    slashingReserve: 0n,
    proof: [],
  });

  beforeEach(() => {
    jest.clearAllMocks();

    logger = createLoggerMock() as jest.Mocked<ILogger>;
    vaultHubClient = createVaultHubClientMock();
    lidoReportClient = createLidoReportClientMock();
    metricsUpdater = createMetricsUpdaterMock();

    // Default mocks for happy path
    vaultHubClient.isVaultConnected.mockResolvedValue(true);
    vaultHubClient.isReportFresh.mockResolvedValue(false);
    lidoReportClient.getLatestSubmitVaultReportParams.mockResolvedValue(createVaultReportParams());
    lidoReportClient.submitLatestVaultReport.mockResolvedValue(undefined);
    attemptMock.mockImplementation(
      ((loggerArg: ILogger, fn: () => unknown | Promise<unknown>) =>
        ResultAsync.fromPromise((async () => fn())(), (error) => error as Error)) as typeof attempt,
    );
  });

  it("skips submission when flag is disabled", async () => {
    // Arrange
    const shouldSubmitVaultReport = false;

    // Act
    await submitVaultReportIfNotFresh(
      logger,
      vaultHubClient,
      lidoReportClient,
      metricsUpdater,
      VAULT_ADDRESS,
      shouldSubmitVaultReport,
      LOG_PREFIX,
    );

    // Assert
    expect(vaultHubClient.isVaultConnected).not.toHaveBeenCalled();
    expect(vaultHubClient.isReportFresh).not.toHaveBeenCalled();
    expect(lidoReportClient.getLatestSubmitVaultReportParams).not.toHaveBeenCalled();
    expect(lidoReportClient.submitLatestVaultReport).not.toHaveBeenCalled();
    expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
    expect(logger.info).toHaveBeenCalledWith(
      `${LOG_PREFIX} - Skipping vault report submission (SHOULD_SUBMIT_VAULT_REPORT=false)`,
    );
  });

  it("skips submission when vault is not connected", async () => {
    // Arrange
    vaultHubClient.isVaultConnected.mockResolvedValue(false);

    // Act
    await submitVaultReportIfNotFresh(
      logger,
      vaultHubClient,
      lidoReportClient,
      metricsUpdater,
      VAULT_ADDRESS,
      true,
      LOG_PREFIX,
    );

    // Assert
    expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(vaultHubClient.isReportFresh).not.toHaveBeenCalled();
    expect(lidoReportClient.getLatestSubmitVaultReportParams).not.toHaveBeenCalled();
    expect(lidoReportClient.submitLatestVaultReport).not.toHaveBeenCalled();
    expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
    expect(logger.info).toHaveBeenCalledWith(`${LOG_PREFIX} - Skipping vault report submission (vault is not connected)`);
  });

  it("proceeds with submission when vault connection check fails", async () => {
    // Arrange
    const connectionError = new Error("Connection check failed");
    vaultHubClient.isVaultConnected.mockRejectedValue(connectionError);

    // Act
    await submitVaultReportIfNotFresh(
      logger,
      vaultHubClient,
      lidoReportClient,
      metricsUpdater,
      VAULT_ADDRESS,
      true,
      LOG_PREFIX,
    );

    // Assert
    expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(logger.warn).toHaveBeenCalledWith(
      `${LOG_PREFIX} - Failed to check if vault is connected, proceeding with submission attempt: ${connectionError}`,
    );
    expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(lidoReportClient.getLatestSubmitVaultReportParams).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(lidoReportClient.submitLatestVaultReport).toHaveBeenCalledWith(VAULT_ADDRESS);
  });

  it("skips submission when report is fresh", async () => {
    // Arrange
    vaultHubClient.isReportFresh.mockResolvedValue(true);

    // Act
    await submitVaultReportIfNotFresh(
      logger,
      vaultHubClient,
      lidoReportClient,
      metricsUpdater,
      VAULT_ADDRESS,
      true,
      LOG_PREFIX,
    );

    // Assert
    expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(lidoReportClient.getLatestSubmitVaultReportParams).not.toHaveBeenCalled();
    expect(lidoReportClient.submitLatestVaultReport).not.toHaveBeenCalled();
    expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
    expect(logger.info).toHaveBeenCalledWith(`${LOG_PREFIX} - Skipping vault report submission (report is fresh)`);
  });

  it("proceeds with submission when freshness check fails", async () => {
    // Arrange
    const freshnessError = new Error("Freshness check failed");
    vaultHubClient.isReportFresh.mockRejectedValue(freshnessError);

    // Act
    await submitVaultReportIfNotFresh(
      logger,
      vaultHubClient,
      lidoReportClient,
      metricsUpdater,
      VAULT_ADDRESS,
      true,
      LOG_PREFIX,
    );

    // Assert
    expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(logger.warn).toHaveBeenCalledWith(
      `${LOG_PREFIX} - Failed to check if report is fresh, proceeding with submission attempt: ${freshnessError}`,
    );
    expect(lidoReportClient.getLatestSubmitVaultReportParams).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(lidoReportClient.submitLatestVaultReport).toHaveBeenCalledWith(VAULT_ADDRESS);
  });

  it("submits vault report when vault is connected and report is stale", async () => {
    // Arrange
    // Default mocks already configured for happy path

    // Act
    await submitVaultReportIfNotFresh(
      logger,
      vaultHubClient,
      lidoReportClient,
      metricsUpdater,
      VAULT_ADDRESS,
      true,
      LOG_PREFIX,
    );

    // Assert
    expect(vaultHubClient.isVaultConnected).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(vaultHubClient.isReportFresh).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(lidoReportClient.getLatestSubmitVaultReportParams).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(lidoReportClient.submitLatestVaultReport).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(logger.info).toHaveBeenCalledWith(`${LOG_PREFIX} - Fetching latest vault report`);
    expect(logger.info).toHaveBeenCalledWith(`${LOG_PREFIX} - Submitting latest vault report`);
  });

  it("increments metrics when submission succeeds", async () => {
    // Arrange
    const successResult = ok(undefined);
    attemptMock.mockResolvedValue(successResult);

    // Act
    await submitVaultReportIfNotFresh(
      logger,
      vaultHubClient,
      lidoReportClient,
      metricsUpdater,
      VAULT_ADDRESS,
      true,
      LOG_PREFIX,
    );

    // Assert
    expect(metricsUpdater.incrementLidoVaultAccountingReport).toHaveBeenCalledWith(VAULT_ADDRESS);
    expect(logger.info).toHaveBeenCalledWith(`${LOG_PREFIX} - Vault report submission succeeded`);
  });

  it("does not increment metrics when submission fails", async () => {
    // Arrange
    const submissionError = new Error("Submission failed");
    const failureResult = err(submissionError);
    attemptMock.mockResolvedValue(failureResult);

    // Act
    await submitVaultReportIfNotFresh(
      logger,
      vaultHubClient,
      lidoReportClient,
      metricsUpdater,
      VAULT_ADDRESS,
      true,
      LOG_PREFIX,
    );

    // Assert
    expect(metricsUpdater.incrementLidoVaultAccountingReport).not.toHaveBeenCalled();
    expect(logger.info).not.toHaveBeenCalledWith(`${LOG_PREFIX} - Vault report submission succeeded`);
  });
});

