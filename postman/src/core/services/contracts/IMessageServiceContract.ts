import { MessageProps } from "../../entities/Message";
import { OnChainMessageStatus } from "../../enums";
import { Address, Hash, ErrorDescription, MessageSent, Overrides, TransactionSubmission } from "../../types";

export interface IMessageStatusReader {
  getMessageStatus(params: { messageHash: Hash; messageBlockNumber?: number }): Promise<OnChainMessageStatus>;
}

export interface IMessageClaimer {
  claim(
    message: (MessageSent | MessageProps) & { feeRecipient?: Address; messageBlockNumber?: number },
    opts?: {
      claimViaAddress?: Address;
      overrides?: Overrides;
    },
  ): Promise<TransactionSubmission>;
}

export interface IRateLimitChecker {
  isRateLimitExceeded(messageFee: bigint, messageValue: bigint): Promise<boolean>;
  isRateLimitExceededError(transactionHash: Hash, blockNumber?: bigint): Promise<boolean>;
}

export interface IContractTransactionErrorParser {
  parseTransactionError(transactionHash: Hash, blockNumber?: bigint): Promise<ErrorDescription | string>;
}

export interface IMessageServiceContract
  extends IMessageStatusReader, IMessageClaimer, IRateLimitChecker, IContractTransactionErrorParser {}
