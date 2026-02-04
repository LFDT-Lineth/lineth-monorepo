import { Address } from "viem";

/**
 * Interface for reporting contract estimateGas errors for metrics tracking.
 */
export interface IEstimateGasErrorReporter {
  /**
   * Records a contract estimateGas error for metrics tracking.
   *
   * @param {Address} contractAddress - The contract address where the error occurred.
   * @param {string} rawRevertData - The raw revert data (hex string).
   * @param {string} [errorName] - The decoded error name (if available).
   */
  recordContractError(contractAddress: Address, rawRevertData: string, errorName?: string): void;
}
