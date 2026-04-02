import { ILogger } from "@consensys/linea-shared-utils";

import { PROFIT_MARGIN_MULTIPLIER } from "../core/constants";
import { Message } from "../core/entities/Message";
import {
  ITransactionValidationService,
  TransactionEvaluation,
  TransactionValidationServiceConfig,
} from "../core/services/ITransactionValidationService";
import { Address } from "../core/types";

export abstract class BaseTransactionValidationService implements ITransactionValidationService {
  constructor(
    protected readonly config: TransactionValidationServiceConfig,
    protected readonly logger: ILogger,
  ) {}

  public abstract evaluateTransaction(
    message: Message,
    feeRecipient?: Address,
    claimViaAddress?: Address,
  ): Promise<TransactionEvaluation>;

  protected hasZeroFee(message: Message): boolean {
    return message.hasZeroFee() && this.config.profitMargin !== 0;
  }

  protected calculateGasEstimationThreshold(messageFee: bigint, gasLimit: bigint): number {
    return parseFloat((messageFee / gasLimit).toString());
  }

  protected getGasLimit(gasLimit: bigint): bigint | null {
    return gasLimit <= this.config.maxClaimGasLimit ? gasLimit : null;
  }

  protected isForSponsorship(gasLimit: bigint, hasZeroFee: boolean, isUnderPriced: boolean): boolean {
    if (!this.config.isPostmanSponsorshipEnabled) return false;
    if (gasLimit > this.config.maxPostmanSponsorGasLimit) return false;
    if (hasZeroFee) return true;
    if (isUnderPriced) return true;
    return false;
  }

  protected logEvaluation(
    messageHash: string,
    gasLimit: bigint,
    maxPriorityFeePerGas: bigint,
    maxFeePerGas: bigint,
    evaluation: TransactionEvaluation,
  ): void {
    this.logger.debug(
      `Estimated gas fees for message claiming. messageHash=${messageHash} gasLimit=${gasLimit} maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas}`,
    );
    this.logger.debug(
      `Transaction evaluation results. messageHash=${messageHash} hasZeroFee=${evaluation.hasZeroFee} isUnderPriced=${evaluation.isUnderPriced} isRateLimitExceeded=${evaluation.isRateLimitExceeded} isForSponsorship=${evaluation.isForSponsorship} estimatedGasLimit=${evaluation.estimatedGasLimit} threshold=${evaluation.threshold}`,
    );
  }

  protected computeIsUnderPricedByMaxFee(gasLimit: bigint, messageFee: bigint, maxFeePerGas: bigint): boolean {
    const actualCost =
      gasLimit * maxFeePerGas * BigInt(Math.floor(this.config.profitMargin * PROFIT_MARGIN_MULTIPLIER));
    const maxFee = messageFee * BigInt(PROFIT_MARGIN_MULTIPLIER);
    return actualCost > maxFee;
  }
}
