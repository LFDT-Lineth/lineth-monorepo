import { Message, MessageSent } from "../types";

export interface IMessageServiceContract<
  Overrides,
  TransactionReceipt,
  TransactionResponse,
  ContractTransactionResponse,
  ErrorDescription,
> {
  getMessageByMessageHash(messageHash: string): Promise<MessageSent | null>;
  getMessagesByTransactionHash(transactionHash: string): Promise<MessageSent[] | null>;
  getTransactionReceiptByMessageHash(messageHash: string): Promise<TransactionReceipt | null>;
  claim(
    message: Message & { feeRecipient?: string },
    opts?: {
      claimViaAddress?: string;
      overrides?: Overrides;
    },
  ): Promise<ContractTransactionResponse>;
  retryTransactionWithHigherFee(transactionHash: string, priceBumpPercent?: number): Promise<TransactionResponse>;
  isRateLimitExceeded(messageFee: bigint, messageValue: bigint): Promise<boolean>;
  isRateLimitExceededError(transactionHash: string): Promise<boolean>;
  parseTransactionError(transactionHash: string): Promise<ErrorDescription | string>;
}
