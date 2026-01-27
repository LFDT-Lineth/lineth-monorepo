import { Direction } from "@consensys/linea-sdk";

import { Message } from "../entities/Message";
import { MessageStatus } from "../enums";

export interface IMessageDBService<TransactionResponse> {
  insertMessage(message: Message): Promise<void>;
  saveMessages(messages: Message[]): Promise<void>;
  updateMessage(message: Message): Promise<void>;
  deleteMessages(msBeforeNowToDelete: number): Promise<number>;
  getFirstPendingMessage(direction: Direction): Promise<Message | null>;
  getLatestMessageSent(direction: Direction, contractAddress: string): Promise<Message | null>;
  getNFirstMessagesSent(limit: number, contractAddress: string): Promise<Message[]>;
  getNFirstMessagesByStatus(
    status: MessageStatus,
    direction: Direction,
    limit: number,
    contractAddress: string,
  ): Promise<Message[]>;
  getMessageToClaim(
    contractAddress: string,
    gasEstimationMargin: number,
    maxRetry: number,
    retryDelay: number,
  ): Promise<Message | null>;
  getLastClaimTxNonce(direction: Direction): Promise<number | null>;
  updateMessageWithClaimTxAtomic(
    message: Message,
    nonce: number,
    claimTxFn: () => Promise<TransactionResponse>,
  ): Promise<void>;
}
