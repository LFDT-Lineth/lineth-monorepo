import { Message } from "../entities/Message";

import type { Address } from "../types/primitives";

export type TransactionEvaluation = {
  hasZeroFee: boolean;
  isUnderPriced: boolean;
  isRateLimitExceeded: boolean;
  isForSponsorship: boolean;
  estimatedGasLimit: bigint | null;
  threshold: number;
  maxPriorityFeePerGas: bigint;
  maxFeePerGas: bigint;
};

export interface ITransactionValidationService {
  evaluateTransaction(
    message: Message,
    feeRecipient?: Address,
    claimViaAddress?: Address,
  ): Promise<TransactionEvaluation>;
}

export type TransactionValidationServiceConfig = {
  profitMargin: number;
  maxClaimGasLimit: bigint;
  isPostmanSponsorshipEnabled: boolean;
  maxPostmanSponsorGasLimit: bigint;
};
