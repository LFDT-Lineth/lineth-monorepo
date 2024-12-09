import { OnChainMessageStatus, MessageSent } from "@consensys/linea-sdk";
import { MessageProps } from "../../entities/Message";

export interface IMessageServiceContract<
  Overrides,
  TransactionReceipt,
  TransactionResponse,
  ContractTransactionResponse,
  ErrorDescription,
> {
  getMessageStatus(messageHash: string, overrides?: Overrides): Promise<OnChainMessageStatus>;
  getMessageByMessageHash(messageHash: string): Promise<MessageSent | null>;
  getMessagesByTransactionHash(transactionHash: string): Promise<MessageSent[] | null>;
  getTransactionReceiptByMessageHash(messageHash: string): Promise<TransactionReceipt | null>;
  claim(
    message: (MessageSent | MessageProps) & { feeRecipient?: string },
    overrides?: Overrides,
  ): Promise<ContractTransactionResponse>;
  retryTransactionWithHigherFee(transactionHash: string, priceBumpPercent?: number): Promise<TransactionResponse>;
  isRateLimitExceeded(messageFee: bigint, messageValue: bigint): Promise<boolean>;
  isRateLimitExceededError(transactionHash: string): Promise<boolean>;
  parseTransactionError(transactionHash: string): Promise<ErrorDescription | string>;
}
