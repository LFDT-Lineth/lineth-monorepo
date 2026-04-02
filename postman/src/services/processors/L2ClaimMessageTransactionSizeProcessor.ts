import { ILogger } from "@consensys/linea-shared-utils";

import { IL2MessageServiceClient } from "../../core/clients/blockchain/linea/IL2MessageServiceClient";
import { Message } from "../../core/entities/Message";
import { MessageStatus } from "../../core/enums";
import { IErrorParser } from "../../core/errors/IErrorParser";
import { IMessageRepository } from "../../core/persistence/IMessageRepository";
import {
  IL2ClaimMessageTransactionSizeProcessor,
  L2ClaimMessageTransactionSizeProcessorConfig,
} from "../../core/services/processors/IL2ClaimMessageTransactionSizeProcessor";
import { IL2ClaimTransactionSizeCalculator } from "../../core/services/processors/IL2ClaimTransactionSizeCalculator";

export class L2ClaimMessageTransactionSizeProcessor implements IL2ClaimMessageTransactionSizeProcessor {
  /**
   * Constructs a new instance of the `L2ClaimMessageTransactionSizeProcessor`.
   *
   * @param {IMessageRepository} messageRepository - The message repository for interacting with message data.
   * @param {IL2MessageServiceClient} l2MessageServiceClient - The L2 message service client for estimating gas fees.
   * @param {IL2ClaimTransactionSizeCalculator} transactionSizeCalculator - The calculator for determining the transaction size.
   * @param {L2ClaimMessageTransactionSizeProcessorConfig} config - Configuration settings for the processor, including the direction and origin contract address.
   * @param {ILogger} logger - The logger for logging information and errors.
   */
  constructor(
    private readonly messageRepository: IMessageRepository,
    private readonly l2MessageServiceClient: IL2MessageServiceClient,
    private readonly transactionSizeCalculator: IL2ClaimTransactionSizeCalculator,
    private readonly config: L2ClaimMessageTransactionSizeProcessorConfig,
    private readonly logger: ILogger,
    private readonly errorParser: IErrorParser,
  ) {}

  /**
   * Processes the transaction size and gas limit for L2 claim messages.
   * Fetches the first anchored message, calculates its transaction size and gas limit, updates the message status, and logs the information.
   *
   * @returns {Promise<void>} A promise that resolves when the processing is complete.
   */
  public async process(): Promise<void> {
    let message: Message | null = null;

    try {
      const messages = await this.messageRepository.getNFirstMessagesByStatus(
        MessageStatus.ANCHORED,
        this.config.direction,
        1,
        this.config.originContractAddress,
      );

      if (messages.length === 0) {
        this.logger.debug("No anchored messages found to compute transaction size.");
        return;
      }

      message = messages[0];

      this.logger.debug("Computing transaction size.", { messageHash: message.messageHash });

      const { gasLimit, maxPriorityFeePerGas, maxFeePerGas } =
        await this.l2MessageServiceClient.estimateClaimGasFees(message);

      const transactionSize = await this.transactionSizeCalculator.calculateTransactionSize(message, {
        maxPriorityFeePerGas,
        maxFeePerGas,
        gasLimit,
      });

      message.edit({
        claimTxGasLimit: Number(gasLimit),
        compressedTransactionSize: transactionSize,
        status: MessageStatus.TRANSACTION_SIZE_COMPUTED,
      });

      await this.messageRepository.updateMessage(message);

      this.logger.info("Message transaction size and gas limit have been computed.", {
        messageHash: message.messageHash,
        transactionSize,
        gasLimit: gasLimit.toString(),
      });
    } catch (e) {
      await this.handleProcessingError(e, message);
    }
  }

  /**
   * Handles error that occur during the processing.
   *
   * @param {unknown} e - The error that occurred.
   * @param {Message | null} message - The message object being processed when the error occurred.
   * @returns {Promise<void>} A promise that resolves when the error has been handled.
   */
  private async handleProcessingError(e: unknown, message: Message | null): Promise<void> {
    const parsedError = this.errorParser.parse(e);

    if (!parsedError.retryable && message) {
      message.edit({ status: MessageStatus.NON_EXECUTABLE });
      await this.messageRepository.updateMessage(message);
      this.logger.error("Non-retryable error computing transaction size.", {
        error: e,
        parsedError,
        messageHash: message.messageHash,
      });
      return;
    }

    this.logger.error("Error computing transaction size.", {
      error: e,
      parsedError,
    });
  }
}
