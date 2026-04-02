import { ILogger } from "@consensys/linea-shared-utils";
import { type PublicClient } from "viem";
import { estimateGas as lineaEstimateGas } from "viem/linea";

import {
  ILineaGasProvider,
  LineaGasFees,
  LineaGasProviderConfig,
} from "../../../../core/clients/blockchain/IGasProvider";
import { DEFAULT_GAS_ESTIMATION_PERCENTILE, FEE_HISTORY_BLOCK_COUNT } from "../../../../core/constants";
import { BaseError } from "../../../../core/errors";
import { TransactionRequest } from "../../../../core/types";

export class ViemLineaGasProvider implements ILineaGasProvider {
  constructor(
    private readonly client: PublicClient,
    private readonly config: LineaGasProviderConfig,
    private readonly logger: ILogger,
  ) {}

  public async getGasFees(transactionRequest: TransactionRequest): Promise<LineaGasFees> {
    if (!transactionRequest.from) {
      throw new BaseError("ViemLineaGasProvider: Transaction request must specify the 'from' address.");
    }

    if (this.config.enableLineaEstimateGas === false) {
      return this.getGasFeesViaStandardEstimation(transactionRequest);
    }

    return this.getGasFeesViaLineaEstimation(transactionRequest);
  }

  private async getGasFeesViaLineaEstimation(transactionRequest: TransactionRequest): Promise<LineaGasFees> {
    const { gasLimit, baseFeePerGas, priorityFeePerGas } = await lineaEstimateGas(this.client, {
      account: transactionRequest.from!,
      to: transactionRequest.to,
      data: transactionRequest.data,
      value: transactionRequest.value,
    });

    return this.applyFeeCap(gasLimit, baseFeePerGas + priorityFeePerGas, priorityFeePerGas);
  }

  private async getGasFeesViaStandardEstimation(transactionRequest: TransactionRequest): Promise<LineaGasFees> {
    const [gasLimit, feeHistory] = await Promise.all([
      this.client.estimateGas({
        account: transactionRequest.from!,
        to: transactionRequest.to,
        data: transactionRequest.data,
        value: transactionRequest.value,
      }),
      this.client.getFeeHistory({
        blockCount: FEE_HISTORY_BLOCK_COUNT,
        blockTag: "latest",
        rewardPercentiles: [this.config.gasEstimationPercentile ?? DEFAULT_GAS_ESTIMATION_PERCENTILE],
      }),
    ]);

    const rewards = feeHistory.reward ?? [];
    const maxPriorityFeePerGas =
      rewards.length > 0 ? rewards.reduce((acc, r) => acc + r[0], 0n) / BigInt(rewards.length) : 0n;

    const latestBaseFee = feeHistory.baseFeePerGas[feeHistory.baseFeePerGas.length - 1];
    const rawMaxFeePerGas = latestBaseFee * 2n + maxPriorityFeePerGas;

    this.logger.debug("Gas fees estimated via standard eth_estimateGas + getFeeHistory.", {
      gasLimit: gasLimit.toString(),
      maxPriorityFeePerGas: maxPriorityFeePerGas.toString(),
      rawMaxFeePerGas: rawMaxFeePerGas.toString(),
    });

    return this.applyFeeCap(gasLimit, rawMaxFeePerGas, maxPriorityFeePerGas);
  }

  private applyFeeCap(gasLimit: bigint, rawMaxFeePerGas: bigint, maxPriorityFeePerGas: bigint): LineaGasFees {
    const capped = this.config.enforceMaxGasFee && rawMaxFeePerGas > this.config.maxFeePerGasCap;
    const maxFeePerGas = capped ? this.config.maxFeePerGasCap : rawMaxFeePerGas;

    if (capped) {
      this.logger.debug("Gas fee capped to maxFeePerGasCap.", {
        rawMaxFeePerGas: rawMaxFeePerGas.toString(),
        cappedMaxFeePerGas: maxFeePerGas.toString(),
      });
    }

    return { gasLimit, maxFeePerGas, maxPriorityFeePerGas };
  }

  public getMaxFeePerGas(): bigint {
    return this.config.maxFeePerGasCap;
  }
}
