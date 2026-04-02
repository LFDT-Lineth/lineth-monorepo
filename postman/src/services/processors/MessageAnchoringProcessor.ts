import { ILogger } from "@consensys/linea-shared-utils";

import { OnChainMessageStatus, MessageStatus } from "../../core/enums";
import { IMessageRepository } from "../../core/persistence/IMessageRepository";
import { IMessageStatusReader } from "../../core/services/contracts/IMessageServiceContract";
import {
  IMessageAnchoringProcessor,
  MessageAnchoringProcessorConfig,
} from "../../core/services/processors/IMessageAnchoringProcessor";

export class MessageAnchoringProcessor implements IMessageAnchoringProcessor {
  private readonly maxFetchMessagesFromDb: number;

  /**
   * Constructs a new instance of the `MessageAnchoringProcessor`.
   *
   * @param {IMessageStatusReader} contractClient - Used to check on-chain message status.
   * @param {IMessageRepository} messageRepository - An instance of a class implementing the `IMessageRepository` interface, used for storing and retrieving message data.
   * @param {MessageAnchoringProcessorConfig} config - Configuration settings for the processor, including the maximum number of messages to fetch from the database for processing.
   * @param {ILogger} logger - An instance of a class implementing the `ILogger` interface, used for logging messages.
   */
  constructor(
    private readonly contractClient: IMessageStatusReader,
    private readonly messageRepository: IMessageRepository,
    private readonly config: MessageAnchoringProcessorConfig,
    private readonly logger: ILogger,
  ) {
    this.maxFetchMessagesFromDb = Math.max(config.maxFetchMessagesFromDb, 0);
  }

  /**
   * Fetches a set number of messages from the database and updates their status based on the latest anchoring information from the blockchain.
   *
   * @returns {Promise<void>} A promise that resolves when the processing is complete.
   */
  public async process(): Promise<void> {
    try {
      const messages = await this.messageRepository.getNFirstMessagesByStatus(
        MessageStatus.SENT,
        this.config.direction,
        this.maxFetchMessagesFromDb,
        this.config.originContractAddress,
      );

      if (messages.length === this.maxFetchMessagesFromDb) {
        this.logger.warn("Limit of messages sent to listen reached.", { limit: this.maxFetchMessagesFromDb });
      }

      if (messages.length === 0) {
        this.logger.debug("No messages to process for anchoring.");
        return;
      }

      this.logger.debug("Fetched messages for anchoring.", {
        count: messages.length,
        direction: this.config.direction,
      });

      for (const message of messages) {
        this.logger.debug("Checking on-chain status.", { messageHash: message.messageHash });

        const messageStatus = await this.contractClient.getMessageStatus({
          messageHash: message.messageHash,
          messageBlockNumber: message.sentBlockNumber,
        });

        this.logger.debug("Fetched on-chain message status.", {
          messageHash: message.messageHash,
          status: messageStatus,
        });

        if (messageStatus === OnChainMessageStatus.CLAIMABLE) {
          message.edit({ status: MessageStatus.ANCHORED });
          this.logger.info("Message has been anchored.", { messageHash: message.messageHash });
        }

        if (messageStatus === OnChainMessageStatus.CLAIMED) {
          message.edit({ status: MessageStatus.CLAIMED_SUCCESS });
          this.logger.info("Message has already been claimed.", { messageHash: message.messageHash });
        }
      }

      await this.messageRepository.saveMessages(messages);
    } catch (e) {
      this.logger.error("Error processing message anchoring.", {
        error: e,
        direction: this.config.direction,
      });
    }
  }
}
