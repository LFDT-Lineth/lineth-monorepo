import { ILogger } from "@consensys/linea-shared-utils";

import { BaseTransactionValidationService } from "./BaseTransactionValidationService";
import { IL2MessageServiceClient } from "../core/clients/blockchain/linea/IL2MessageServiceClient";
import { ILineaProvider } from "../core/clients/blockchain/linea/ILineaProvider";
import { MINIMUM_MARGIN, PROFIT_MARGIN_MULTIPLIER } from "../core/constants";
import { Message } from "../core/entities/Message";
import { BaseError } from "../core/errors";
import {
  TransactionEvaluation,
  TransactionValidationServiceConfig,
} from "../core/services/ITransactionValidationService";
import { Address } from "../core/types";

export class LineaTransactionValidationService extends BaseTransactionValidationService {
  constructor(
    config: TransactionValidationServiceConfig,
    private readonly provider: ILineaProvider,
    private readonly l2MessageServiceClient: IL2MessageServiceClient,
    logger: ILogger,
  ) {
    super(config, logger);
  }

  public async evaluateTransaction(
    message: Message,
    feeRecipient?: Address,
    claimViaAddress?: Address,
  ): Promise<TransactionEvaluation> {
    const { gasLimit, maxPriorityFeePerGas, maxFeePerGas } = await this.l2MessageServiceClient.estimateClaimGasFees(
      {
        ...message,
        feeRecipient: feeRecipient,
      },
      { claimViaAddress },
    );

    const threshold = this.calculateGasEstimationThreshold(message.fee, gasLimit);
    const estimatedGasLimit = this.getGasLimit(gasLimit);

    if (message.compressedTransactionSize === undefined) {
      throw new BaseError(`compressedTransactionSize is undefined for message. messageHash=${message.messageHash}`);
    }
    const isUnderPriced = await this.computeIsUnderPricedLinea(
      gasLimit,
      message.fee,
      message.compressedTransactionSize,
    );

    const hasZeroFee = this.hasZeroFee(message);
    const isRateLimitExceeded = await this.l2MessageServiceClient.isRateLimitExceeded(message.fee, message.value);
    const isForSponsorship = this.isForSponsorship(gasLimit, hasZeroFee, isUnderPriced);

    const evaluation: TransactionEvaluation = {
      hasZeroFee,
      isUnderPriced,
      isRateLimitExceeded,
      isForSponsorship,
      estimatedGasLimit,
      threshold,
      maxPriorityFeePerGas,
      maxFeePerGas,
    };

    this.logEvaluation(message.messageHash, gasLimit, maxPriorityFeePerGas, maxFeePerGas, evaluation);

    return evaluation;
  }

  private async computeIsUnderPricedLinea(
    gasLimit: bigint,
    messageFee: bigint,
    messageCompressedTransactionSize: number,
  ): Promise<boolean> {
    const extraData = await this.provider.getBlockExtraData("latest");

    if (!extraData) {
      throw new BaseError("No extra data.");
    }

    const priorityFee =
      (BigInt(MINIMUM_MARGIN * 10) *
        ((BigInt(extraData.variableCost) * BigInt(messageCompressedTransactionSize)) / gasLimit +
          BigInt(extraData.fixedCost))) /
      10n;

    const actualCost = priorityFee * gasLimit * BigInt(Math.floor(this.config.profitMargin * PROFIT_MARGIN_MULTIPLIER));

    const maxFee = messageFee * BigInt(PROFIT_MARGIN_MULTIPLIER);
    return maxFee < actualCost;
  }
}
