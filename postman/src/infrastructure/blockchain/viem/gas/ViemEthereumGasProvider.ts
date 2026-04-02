import { ILogger } from "@consensys/linea-shared-utils";
import { type PublicClient } from "viem";

import {
  DefaultGasProviderConfig,
  GasFees,
  IEthereumGasProvider,
} from "../../../../core/clients/blockchain/IGasProvider";
import { FEE_HISTORY_BLOCK_COUNT } from "../../../../core/constants";
import { BaseError } from "../../../../core/errors/BaseError";

export class ViemEthereumGasProvider implements IEthereumGasProvider {
  private gasFeesCache: GasFees;
  private cacheIsValidForBlockNumber: bigint;

  constructor(
    private readonly client: PublicClient,
    private readonly config: DefaultGasProviderConfig,
    private readonly logger: ILogger,
  ) {
    this.cacheIsValidForBlockNumber = 0n;
    this.gasFeesCache = {
      maxFeePerGas: this.config.maxFeePerGasCap,
      maxPriorityFeePerGas: this.config.maxFeePerGasCap,
    };
  }

  public async getGasFees(): Promise<GasFees> {
    if (this.config.enforceMaxGasFee) {
      return {
        maxPriorityFeePerGas: this.config.maxFeePerGasCap,
        maxFeePerGas: this.config.maxFeePerGasCap,
      };
    }

    const currentBlockNumber = await this.client.getBlockNumber();
    if (this.isCacheValid(currentBlockNumber)) {
      return this.gasFeesCache;
    }

    const feeHistory = await this.fetchFeeHistory();
    const maxPriorityFeePerGas = this.calculateMaxPriorityFee(feeHistory.reward ?? []);

    if (maxPriorityFeePerGas > this.config.maxFeePerGasCap) {
      const msg = `Estimated miner tip of ${maxPriorityFeePerGas} exceeds configured max fee per gas of ${this.config.maxFeePerGasCap}!`;
      this.logger.warn(msg);
      throw new BaseError(msg);
    }

    this.updateCache(currentBlockNumber, feeHistory.baseFeePerGas, maxPriorityFeePerGas);
    this.logger.debug("Gas fees updated from fee history.", {
      maxFeePerGas: this.gasFeesCache.maxFeePerGas.toString(),
      maxPriorityFeePerGas: this.gasFeesCache.maxPriorityFeePerGas.toString(),
      blockNumber: currentBlockNumber.toString(),
    });
    return this.gasFeesCache;
  }

  private async fetchFeeHistory() {
    return this.client.getFeeHistory({
      blockCount: FEE_HISTORY_BLOCK_COUNT,
      blockTag: "latest",
      rewardPercentiles: [this.config.gasEstimationPercentile],
    });
  }

  private calculateMaxPriorityFee(reward: bigint[][]): bigint {
    return reward.reduce((acc, currentValue) => acc + currentValue[0], 0n) / BigInt(reward.length);
  }

  private isCacheValid(currentBlockNumber: bigint): boolean {
    return this.cacheIsValidForBlockNumber >= currentBlockNumber;
  }

  private updateCache(currentBlockNumber: bigint, baseFeePerGas: bigint[], maxPriorityFeePerGas: bigint) {
    this.cacheIsValidForBlockNumber = currentBlockNumber;
    const maxFeePerGas = baseFeePerGas[baseFeePerGas.length - 1] * 2n + maxPriorityFeePerGas;
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

  public getMaxFeePerGas(): bigint {
    return this.config.maxFeePerGasCap;
  }
}
