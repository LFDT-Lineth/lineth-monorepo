import { ILogger } from "@consensys/linea-shared-utils";

import { BaseTransactionValidationService } from "./BaseTransactionValidationService";
import { ILineaRollupClient } from "../core/clients/blockchain/ethereum/ILineaRollupClient";
import { IEthereumGasProvider } from "../core/clients/blockchain/IGasProvider";
import { Message } from "../core/entities/Message";
import {
  TransactionEvaluation,
  TransactionValidationServiceConfig,
} from "../core/services/ITransactionValidationService";
import { Address } from "../core/types";

export class EthereumTransactionValidationService extends BaseTransactionValidationService {
  constructor(
    private readonly lineaRollupClient: ILineaRollupClient,
    private readonly gasProvider: IEthereumGasProvider,
    config: TransactionValidationServiceConfig,
    logger: ILogger,
  ) {
    super(config, logger);
  }

  public async evaluateTransaction(
    message: Message,
    feeRecipient?: Address,
    claimViaAddress?: Address,
  ): Promise<TransactionEvaluation> {
    const [gasLimit, { maxPriorityFeePerGas, maxFeePerGas }] = await Promise.all([
      this.lineaRollupClient.estimateClaimGas(
        {
          ...message,
          feeRecipient,
          messageBlockNumber: message.sentBlockNumber,
        },
        { claimViaAddress },
      ),
      this.gasProvider.getGasFees(),
    ]);

    const threshold = this.calculateGasEstimationThreshold(message.fee, gasLimit);
    const estimatedGasLimit = this.getGasLimit(gasLimit);
    const isUnderPriced = this.computeIsUnderPricedByMaxFee(gasLimit, message.fee, maxFeePerGas);
    const hasZeroFee = this.hasZeroFee(message);
    const isRateLimitExceeded = await this.lineaRollupClient.isRateLimitExceeded(message.fee, message.value);
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
}
