import { ErrorParser } from "../../utils/ErrorParser";
import {
  Overrides,
  TransactionResponse,
  ContractTransactionResponse,
  EthersError,
  TransactionReceipt,
  Signer,
  ErrorDescription,
} from "ethers";
import { OnChainMessageStatus } from "@consensys/linea-sdk";
import { MessageStatus } from "../../core/enums";
import {
  IMessageClaimingProcessor,
  MessageClaimingProcessorConfig,
} from "../../core/services/processors/IMessageClaimingProcessor";
import { ILogger } from "../../core/utils/logging/ILogger";
import { IMessageServiceContract } from "../../core/services/contracts/IMessageServiceContract";
import { Message } from "../../core/entities/Message";
import { IMessageDBService } from "../../core/persistence/IMessageDBService";
import { ITransactionValidationService } from "../../core/services/ITransactionValidationService";

export class MessageClaimingProcessor implements IMessageClaimingProcessor {
  private readonly maxNonceDiff: number;

  /**
   * Initializes a new instance of the `MessageClaimingProcessor`.
   *
   * @param {IMessageServiceContract} messageServiceContract - An instance of a class implementing the `IMessageServiceContract` interface, used to interact with the blockchain contract.
   * @param {Signer} signer - An instance of a class implementing the `Signer` interface, used to query blockchain data.
   * @param {IMessageDBService} databaseService - An instance of a class implementing the `IMessageDBService` interface, used for storing and retrieving message data.
   * @param {ITransactionValidationService} transactionValidationService - An instance of a class implementing the `ITransactionValidationService` interface, used for validating transactions.
   * @param {MessageClaimingProcessorConfig} config - Configuration for network-specific settings, including transaction submission timeout, maximum transaction retries, and gas limit.
   * @param {ILogger} logger - An instance of a class implementing the `ILogger` interface, used for logging messages.
   */
  constructor(
    private readonly messageServiceContract: IMessageServiceContract<
      Overrides,
      TransactionReceipt,
      TransactionResponse,
      ContractTransactionResponse,
      ErrorDescription
    >,
    private readonly signer: Signer,
    private readonly databaseService: IMessageDBService<TransactionResponse>,
    private readonly transactionValidationService: ITransactionValidationService,
    private readonly config: MessageClaimingProcessorConfig,
    private readonly logger: ILogger,
  ) {
    this.maxNonceDiff = Math.max(config.maxNonceDiff, 0);
  }

  /**
   * Identifies the next message eligible for claiming and attempts to execute the claim transaction. It considers various factors such as gas estimation, profit margin, and rate limits to decide whether to proceed with the claim.
   *
   * @returns {Promise<void>} A promise that resolves when the processing is complete.
   */
  public async process(): Promise<void> {
    let nextMessageToClaim: Message | null = null;

    try {
      const nonce = await this.getNonce();

      if (!nonce && nonce !== 0) {
        this.logger.error("Nonce returned from getNonce is an invalid value (e.g. null or undefined)");
        return;
      }

      nextMessageToClaim = await this.databaseService.getMessageToClaim(
        this.config.originContractAddress,
        this.config.profitMargin,
        this.config.maxNumberOfRetries,
        this.config.retryDelayInSeconds,
      );

      if (!nextMessageToClaim) {
        return;
      }

      const messageStatus = await this.messageServiceContract.getMessageStatus(nextMessageToClaim.messageHash);

      if (messageStatus === OnChainMessageStatus.CLAIMED) {
        this.logger.info("Found already claimed message: messageHash=%s", nextMessageToClaim.messageHash);

        nextMessageToClaim.edit({ status: MessageStatus.CLAIMED_SUCCESS });
        await this.databaseService.updateMessage(nextMessageToClaim);
        return;
      }

      const { hasZeroFee, isUnderPriced, isRateLimitExceeded, estimatedGasLimit, threshold, ...claimTxFees } =
        await this.transactionValidationService.evaluateTransaction(
          nextMessageToClaim,
          this.config.feeRecipientAddress,
        );

      if (await this.handleZeroFee(hasZeroFee, nextMessageToClaim)) return;
      if (await this.handleNonExecutable(nextMessageToClaim, estimatedGasLimit)) return;

      nextMessageToClaim.edit({ claimGasEstimationThreshold: threshold });
      await this.databaseService.updateMessage(nextMessageToClaim);

      if (await this.handleUnderpriced(nextMessageToClaim, isUnderPriced, estimatedGasLimit, claimTxFees.maxFeePerGas))
        return;
      if (this.handleRateLimitExceeded(nextMessageToClaim, isRateLimitExceeded)) return;

      await this.executeClaimTransaction(
        nextMessageToClaim,
        nonce,
        estimatedGasLimit!,
        claimTxFees.maxPriorityFeePerGas,
        claimTxFees.maxFeePerGas,
      );
    } catch (e) {
      await this.handleProcessingError(e, nextMessageToClaim);
    }
  }

  /**
   * Retrieves the current nonce for the claiming transactions, ensuring it is within an acceptable range compared to the last recorded nonce.
   *
   * @returns {Promise<number | null>} The nonce to use for the next transaction, or null if the nonce difference exceeds the configured maximum.
   */
  private async getNonce(): Promise<number | null> {
    const lastTxNonce = await this.databaseService.getLastClaimTxNonce(this.config.direction);

    let nonce = await this.signer.getNonce();
    if (lastTxNonce) {
      if (lastTxNonce - nonce > this.maxNonceDiff) {
        this.logger.warn(
          "Last recorded nonce in db is higher than the latest nonce from blockchain and exceeds the diff limit, paused the claim message process now: nonceInDb=%s nonceOnChain=%s maxAllowedNonceDiff=%s",
          lastTxNonce,
          nonce,
          this.maxNonceDiff,
        );
        return null;
      }
      nonce = Math.max(nonce, lastTxNonce + 1);
    }
    return nonce;
  }

  /**
   * Executes the claim transaction for a message, updating the message repository with transaction details.
   *
   * @param {Message} message - The message object to claim.
   * @param {number} nonce - The nonce to use for the transaction.
   * @param {bigint} gasLimit - The gas limit for the transaction.
   * @param {bigint} maxPriorityFeePerGas - The maximum priority fee per gas for the transaction.
   * @param {bigint} maxFeePerGas - The maximum fee per gas for the transaction.
   * @returns {Promise<void>} A promise that resolves when the transaction is executed.
   */
  private async executeClaimTransaction(
    message: Message,
    nonce: number,
    gasLimit: bigint,
    maxPriorityFeePerGas: bigint,
    maxFeePerGas: bigint,
  ): Promise<void> {
    const claimTxResponsePromise = this.messageServiceContract.claim(
      {
        ...message,
        feeRecipient: this.config.feeRecipientAddress,
      },
      { nonce, gasLimit, maxPriorityFeePerGas, maxFeePerGas },
    );
    await this.databaseService.updateMessageWithClaimTxAtomic(message, nonce, claimTxResponsePromise);
  }

  /**
   * Handles messages with zero fee, updating their status and logging a warning.
   *
   * @param {boolean} hasZeroFee - Indicates whether the message has zero fee.
   * @param {Message} message - The message object to handle.
   * @returns {Promise<boolean>} A promise that resolves to `true` if the message has zero fee, `false` otherwise.
   */
  private async handleZeroFee(hasZeroFee: boolean, message: Message): Promise<boolean> {
    if (hasZeroFee) {
      this.logger.warn(
        "Found message with zero fee. This message will not be processed: messageHash=%s",
        message.messageHash,
      );
      message.edit({ status: MessageStatus.ZERO_FEE });
      await this.databaseService.updateMessage(message);
      return true;
    }
    return false;
  }

  /**
   * Handles non-executable messages, updating their status and logging a warning.
   *
   * @param {Message} message - The message object to handle.
   * @param {bigint | null} estimatedGasLimit - The estimated gas limit for the transaction.
   * @returns {Promise<boolean>} A promise that resolves to `true` if the message is non-executable, `false` otherwise.
   */
  private async handleNonExecutable(message: Message, estimatedGasLimit: bigint | null): Promise<boolean> {
    if (!estimatedGasLimit) {
      this.logger.warn(
        "Estimated gas limit is higher than the max allowed gas limit for this message: messageHash=%s messageInfo=%s estimatedGasLimit=%s maxAllowedGasLimit=%s",
        message.messageHash,
        message.toString(),
        // TODO: fix this
        estimatedGasLimit?.toString(),
        this.config.maxClaimGasLimit.toString(),
      );
      message.edit({ status: MessageStatus.NON_EXECUTABLE });
      await this.databaseService.updateMessage(message);
      return true;
    }
    return false;
  }
  /**
   * Handles underpriced messages, updating their status and logging a warning.
   *
   * @param {Message} message - The message object to handle.
   * @param {boolean} isUnderPriced - Indicates whether the message is underpriced.
   * @param {bigint | null} estimatedGasLimit - The estimated gas limit for the transaction.
   * @param {bigint} maxFeePerGas - The maximum fee per gas for the transaction.
   * @returns {Promise<boolean>} A promise that resolves to `true` if the message is underpriced, `false` otherwise.
   */
  private async handleUnderpriced(
    message: Message,
    isUnderPriced: boolean,
    estimatedGasLimit: bigint | null,
    maxFeePerGas: bigint,
  ): Promise<boolean> {
    if (isUnderPriced) {
      this.logger.warn(
        "Fee underpriced found in this message: messageHash=%s messageInfo=%s transactionGasLimit=%s maxFeePerGas=%s",
        message.messageHash,
        message.toString(),
        estimatedGasLimit?.toString(),
        maxFeePerGas.toString(),
      );
      message.edit({ status: MessageStatus.FEE_UNDERPRICED });
      await this.databaseService.updateMessage(message);
      return true;
    }
    return false;
  }

  /**
   * Handles messages that have exceeded the rate limit, logging a warning.
   *
   * @param {Message} message - The message object to handle.
   * @param {boolean} isRateLimitExceeded - Indicates whether the rate limit has been exceeded.
   * @returns {boolean} `true` if the rate limit has been exceeded, `false` otherwise.
   */
  private handleRateLimitExceeded(message: Message, isRateLimitExceeded: boolean): boolean {
    if (isRateLimitExceeded) {
      this.logger.warn(
        "Rate limit exceeded for this message. It will be reprocessed later: messageHash=%s",
        message.messageHash,
      );
      return true;
    }
    return false;
  }

  /**
   * Handles errors that occur during the processing of messages, updating their status if necessary and logging the error.
   *
   * @param {unknown} e - The error that occurred.
   * @param {Message | null} message - The message object being processed when the error occurred.
   * @returns {Promise<void>} A promise that resolves when the error has been handled.
   */
  private async handleProcessingError(e: unknown, message: Message | null): Promise<void> {
    const parsedError = ErrorParser.parseErrorWithMitigation(e as EthersError);

    if (parsedError?.mitigation && !parsedError.mitigation.shouldRetry && message) {
      message.edit({ status: MessageStatus.NON_EXECUTABLE });
      await this.databaseService.updateMessage(message);
    }

    this.logger.warnOrError(e, {
      parsedError,
    });
  }
}
