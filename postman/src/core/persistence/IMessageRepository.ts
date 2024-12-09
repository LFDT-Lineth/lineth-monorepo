import { Direction } from "@consensys/linea-sdk";
import { Message } from "../entities/Message";
import { MessageStatus } from "../enums";

export interface IMessageRepository<ContractTransactionResponse> {
  insertMessage(message: Message): Promise<void>;
  updateMessage(message: Message): Promise<void>;
  updateMessageByTransactionHash(transactionHash: string, direction: Direction, message: Message): Promise<void>;
  saveMessages(messages: Message[]): Promise<void>;
  deleteMessages(msBeforeNowToDelete: number): Promise<number>;
  getFirstMessageToClaimOnL1(
    direction: Direction,
    contractAddress: string,
    currentGasPrice: bigint,
    gasEstimationMargin: number,
    maxRetry: number,
    retryDelay: number,
  ): Promise<Message | null>;
  getFirstMessageToClaimOnL2(
    direction: Direction,
    contractAddress: string,
    messageStatuses: MessageStatus[],
    maxRetry: number,
    retryDelay: number,
    feeEstimationOptions: {
      minimumMargin: number;
      extraDataVariableCost: number;
      extraDataFixedCost: number;
    },
  ): Promise<Message | null>;
  getLatestMessageSent(direction: Direction, contractAddress: string): Promise<Message | null>;
  getNFirstMessagesByStatus(
    status: MessageStatus,
    direction: Direction,
    limit: number,
    contractAddress: string,
  ): Promise<Message[]>;
  getMessageSent(direction: Direction, contractAddress: string): Promise<Message | null>;
  getLastClaimTxNonce(direction: Direction): Promise<number | null>;
  getFirstPendingMessage(direction: Direction): Promise<Message | null>;
  updateMessageWithClaimTxAtomic(
    message: Message,
    nonce: number,
    claimTxResponsePromise: Promise<ContractTransactionResponse>,
  ): Promise<void>;
}
