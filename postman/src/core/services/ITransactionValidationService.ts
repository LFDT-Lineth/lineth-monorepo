import { Message } from "../entities/Message";

export interface ITransactionValidationService {
  evaluateTransaction(
    message: Message,
    feeRecipient?: string,
  ): Promise<{
    hasZeroFee: boolean;
    isUnderPriced: boolean;
    isRateLimitExceeded: boolean;
    estimatedGasLimit: bigint | null;
    threshold: number;
    maxPriorityFeePerGas: bigint;
    maxFeePerGas: bigint;
  }>;
}

export type TransactionValidationServiceConfig = {
  profitMargin: number;
  maxClaimGasLimit: bigint;
};
