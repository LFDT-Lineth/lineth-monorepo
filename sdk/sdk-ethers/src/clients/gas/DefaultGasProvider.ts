import { Block, TransactionReceipt, TransactionRequest, TransactionResponse } from "ethers";

import { DefaultGasProviderConfig, FeeHistory, GasFees, IEthereumGasProvider } from "../../core/clients/IGasProvider";
import { IProvider } from "../../core/clients/IProvider";
import { makeBaseError } from "../../core/errors/utils";
import { BrowserProvider, LineaBrowserProvider, LineaProvider, Provider } from "../providers";

export class DefaultGasProvider implements IEthereumGasProvider<TransactionRequest> {
  private gasFeesCache: GasFees;
  private cacheIsValidForBlockNumber: bigint;

  /**
   * Creates an instance of DefaultGasProvider.
   *
   * @param {IProvider} provider - The provider for interacting with the blockchain.
   * @param {DefaultGasProviderConfig} config - The configuration for the gas provider.
   */
  constructor(
    protected readonly provider: IProvider<
      TransactionReceipt,
      Block,
      TransactionRequest,
      TransactionResponse,
      Provider | LineaProvider | LineaBrowserProvider | BrowserProvider
    >,
    private readonly config: DefaultGasProviderConfig,
  ) {
    this.cacheIsValidForBlockNumber = 0n;
    this.gasFeesCache = {
      maxFeePerGas: this.config.maxFeePerGasCap,
      maxPriorityFeePerGas: this.config.maxFeePerGasCap,
    };
  }

  /**
   * Fetches EIP-1559 gas fee estimates.
   *
   * This method uses the `eth_feeHistory` RPC endpoint to fetch historical gas fee data and calculates the
   * `maxPriorityFeePerGas` and `maxFeePerGas` based on the specified percentile. If `isMaxGasFeeEnforced` is true,
   * it returns the `maxFeePerGas` as configured in the constructor. Otherwise, it calculates the fees based on
   * the network conditions.
   *
   * The method caches the fee estimates and only fetches new data if the current block number has changed since
   * the last fetch. This reduces the number of RPC calls made to fetch fee data.
   *
   * @param {number} [percentile=this.gasEstimationPercentile] - The percentile value to sample from each block's effective priority fees.
   * @returns {Promise<Fees>} A promise that resolves to an object containing the `maxPriorityFeePerGas` and the `maxFeePerGas`.
   */
  public async getGasFees(): Promise<GasFees> {
    if (this.config.enforceMaxGasFee) {
      return {
        maxPriorityFeePerGas: this.config.maxFeePerGasCap,
        maxFeePerGas: this.config.maxFeePerGasCap,
      };
    }

    const currentBlockNumber = await this.provider.getBlockNumber();
    if (this.isCacheValid(currentBlockNumber)) {
      return this.gasFeesCache;
    }

    const feeHistory = await this.fetchFeeHistory();
    const maxPriorityFeePerGas = this.calculateMaxPriorityFee(feeHistory.reward);

    if (maxPriorityFeePerGas > this.config.maxFeePerGasCap) {
      throw makeBaseError(
        `Estimated miner tip of ${maxPriorityFeePerGas} exceeds configured max fee per gas of ${this.config.maxFeePerGasCap}!`,
      );
    }

    this.updateCache(currentBlockNumber, feeHistory.baseFeePerGas, maxPriorityFeePerGas);
    return this.gasFeesCache;
  }

  /**
   * Fetches the fee history from the blockchain.
   *
   * @private
   * @returns {Promise<FeeHistory>} A promise that resolves to the fee history.
   */
  private async fetchFeeHistory(): Promise<FeeHistory> {
    return this.provider.send("eth_feeHistory", ["0x4", "latest", [this.config.gasEstimationPercentile]]);
  }

  /**
   * Calculates the maximum priority fee based on the reward data.
   *
   * @private
   * @param {string[][]} reward - The reward data from the fee history.
   * @returns {bigint} The calculated maximum priority fee.
   */
  private calculateMaxPriorityFee(reward: string[][]): bigint {
    return (
      reward.reduce((acc: bigint, currentValue: string[]) => acc + BigInt(currentValue[0]), 0n) / BigInt(reward.length)
    );
  }

  /**
   * Checks if the cached gas fees are still valid based on the current block number.
   *
   * @private
   * @param {number} currentBlockNumber - The current block number.
   * @returns {boolean} True if the cache is valid, false otherwise.
   */
  private isCacheValid(currentBlockNumber: number): boolean {
    return this.cacheIsValidForBlockNumber >= BigInt(currentBlockNumber);
  }

  /**
   * Updates the gas fees cache with new data.
   *
   * @private
   * @param {number} currentBlockNumber - The current block number.
   * @param {string[]} baseFeePerGas - The base fee per gas from the fee history.
   * @param {bigint} maxPriorityFeePerGas - The calculated maximum priority fee.
   */
  private updateCache(currentBlockNumber: number, baseFeePerGas: string[], maxPriorityFeePerGas: bigint) {
    this.cacheIsValidForBlockNumber = BigInt(currentBlockNumber);
    const maxFeePerGas = BigInt(baseFeePerGas[baseFeePerGas.length - 1]) * 2n + maxPriorityFeePerGas;
    if (maxFeePerGas > 0n && maxPriorityFeePerGas > 0n) {
      this.gasFeesCache = {
        maxPriorityFeePerGas,
        maxFeePerGas: maxFeePerGas > this.config.maxFeePerGasCap ? this.config.maxFeePerGasCap : maxFeePerGas,
      };
    } else {
      this.gasFeesCache = {
        maxPriorityFeePerGas: this.config.maxFeePerGasCap,
        maxFeePerGas: this.config.maxFeePerGasCap,
      };
    }
  }

  /**
   * Gets the maximum fee per gas as configured.
   *
   * @returns {bigint} The maximum fee per gas.
   */
  public getMaxFeePerGas(): bigint {
    return this.config.maxFeePerGasCap;
  }
}
