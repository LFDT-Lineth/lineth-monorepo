import { Message } from "../entities/Message";
import { Direction } from "../enums";
import { MessageStatus } from "../enums";
import { Address, Hash } from "../types";

export interface IMessageWriter {
  insertMessage(message: Message): Promise<void>;
  updateMessage(message: Message): Promise<void>;
  updateMessageByTransactionHash(transactionHash: Hash, direction: Direction, message: Message): Promise<void>;
  saveMessages(messages: Message[]): Promise<void>;
  deleteMessages(msBeforeNowToDelete: number): Promise<number>;
}

export interface IMessageReader {
  getFirstMessageToClaimOnL1(
    direction: Direction,
    contractAddress: Address,
    currentGasPrice: bigint,
    gasEstimationMargin: number,
    maxRetry: number,
    retryDelay: number,
  ): Promise<Message | null>;
  getFirstMessageToClaimOnL2(
    direction: Direction,
    contractAddress: Address,
    messageStatuses: MessageStatus[],
    maxRetry: number,
    retryDelay: number,
  ): Promise<Message | null>;
  getLatestMessageSent(direction: Direction, contractAddress: Address): Promise<Message | null>;
  getNFirstMessagesByStatus(
    status: MessageStatus,
    direction: Direction,
    limit: number,
    contractAddress: Address,
  ): Promise<Message[]>;
  getMessageSent(direction: Direction, contractAddress: Address): Promise<Message | null>;
  getFirstPendingMessage(direction: Direction): Promise<Message | null>;
  getMaxPendingNonce(direction: Direction): Promise<number | null>;
}

export interface IMessageRepository extends IMessageWriter, IMessageReader {}
