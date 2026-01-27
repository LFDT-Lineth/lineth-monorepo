import { Direction } from "@consensys/linea-sdk";
import { ContractTransactionResponse } from "ethers";

import { Message } from "../../core/entities/Message";
import { MessageStatus } from "../../core/enums";
import { IMessageRepository } from "../../core/persistence/IMessageRepository";

export abstract class MessageDBService {
  /**
   * Creates an instance of `MessageDBService`.
   *
   * @param {IMessageRepository} messageRepository - The message repository for interacting with the message database.
   */
  constructor(protected readonly messageRepository: IMessageRepository<ContractTransactionResponse>) {}

  /**
   * Inserts a message into the database.
   *
   * @param {Message} message - The message to insert.
   * @returns {Promise<void>} A promise that resolves when the message is inserted.
   */
  public async insertMessage(message: Message): Promise<void> {
    return this.messageRepository.insertMessage(message);
  }

  /**
   * Saves multiple messages into the database.
   *
   * @param {Message[]} messages - The messages to save.
   * @returns {Promise<void>} A promise that resolves when the messages are saved.
   */
  public async saveMessages(messages: Message[]): Promise<void> {
    return this.messageRepository.saveMessages(messages);
  }

  /**
   * Updates a message in the database.
   *
   * @param {Message} message - The message to update.
   * @returns {Promise<void>} A promise that resolves when the message is updated.
   */
  public async updateMessage(message: Message): Promise<void> {
    return this.messageRepository.updateMessage(message);
  }

  /**
   * Deletes messages older than a specified time.
   *
   * @param {number} msBeforeNowToDelete - The time in milliseconds before now to delete messages.
   * @returns {Promise<number>} A promise that resolves to the number of deleted messages.
   */
  public async deleteMessages(msBeforeNowToDelete: number): Promise<number> {
    return this.messageRepository.deleteMessages(msBeforeNowToDelete);
  }

  /**
   * Retrieves the first pending message in a given direction.
   *
   * @param {Direction} direction - The direction to filter messages by.
   * @returns {Promise<Message | null>} A promise that resolves to the first pending message, or null if no message is found.
   */
  public async getFirstPendingMessage(direction: Direction): Promise<Message | null> {
    return this.messageRepository.getFirstPendingMessage(direction);
  }

  /**
   * Retrieves the latest message sent in a given direction and contract address.
   *
   * @param {Direction} direction - The direction to filter messages by.
   * @param {string} contractAddress - The contract address to filter messages by.
   * @returns {Promise<Message | null>} A promise that resolves to the latest message sent, or null if no message is found.
   */
  public async getLatestMessageSent(direction: Direction, contractAddress: string): Promise<Message | null> {
    return this.messageRepository.getLatestMessageSent(direction, contractAddress);
  }

  /**
   * Retrieves the first N messages with a given status, direction, and contract address.
   *
   * @param {MessageStatus} status - The status to filter messages by.
   * @param {Direction} direction - The direction to filter messages by.
   * @param {number} limit - The maximum number of messages to retrieve.
   * @param {string} contractAddress - The contract address to filter messages by.
   * @returns {Promise<Message[]>} A promise that resolves to an array of messages.
   */
  public async getNFirstMessagesByStatus(
    status: MessageStatus,
    direction: Direction,
    limit: number,
    contractAddress: string,
  ): Promise<Message[]> {
    return this.messageRepository.getNFirstMessagesByStatus(status, direction, limit, contractAddress);
  }

  /**
   * Retrieves the last claim transaction nonce in a given direction.
   *
   * @param {Direction} direction - The direction to filter messages by.
   * @returns {Promise<number | null>} A promise that resolves to the last claim transaction nonce, or null if no nonce is found.
   */
  public async getLastClaimTxNonce(direction: Direction): Promise<number | null> {
    return this.messageRepository.getLastClaimTxNonce(direction);
  }

  /**
   * Updates a message with a claim transaction atomically.
   *
   * @param {Message} message - The message to update.
   * @param {number} nonce - The nonce to use for the claim transaction.
   * @param {Promise<ContractTransactionResponse>} claimTxFn - Function that resolves to the claim transaction response.
   * @returns {Promise<void>} A promise that resolves when the message is updated.
   */
  public async updateMessageWithClaimTxAtomic(
    message: Message,
    nonce: number,
    claimTxFn: () => Promise<ContractTransactionResponse>,
  ): Promise<void> {
    await this.messageRepository.updateMessageWithClaimTxAtomic(message, nonce, claimTxFn);
  }
}
