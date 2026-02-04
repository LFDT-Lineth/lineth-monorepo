import { Address } from "viem";
import { IEstimateGasErrorReporter } from "@consensys/linea-shared-utils";
import { INativeYieldAutomationMetricsUpdater } from "../metrics/INativeYieldAutomationMetricsUpdater.js";

/**
 * Implementation of IEstimateGasErrorReporter that records contract estimateGas errors
 * via the NativeYieldAutomationMetricsUpdater.
 */
export class EstimateGasErrorReporter implements IEstimateGasErrorReporter {
  constructor(private readonly metricsUpdater: INativeYieldAutomationMetricsUpdater) {}

  /**
   * Records a contract estimateGas error for metrics tracking.
   *
   * @param {Address} contractAddress - The contract address where the error occurred.
   * @param {string} rawRevertData - The raw revert data (hex string).
   * @param {string} [errorName] - The decoded error name (if available).
   */
  recordContractError(contractAddress: Address, rawRevertData: string, errorName?: string): void {
    this.metricsUpdater.incrementContractEstimateGasError(contractAddress, rawRevertData, errorName);
  }
}

