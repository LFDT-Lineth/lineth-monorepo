import { ILogger } from "@consensys/linea-shared-utils";

import { Message } from "../../core/entities/Message";
import { OnChainMessageStatus, MessageStatus } from "../../core/enums";
import { IErrorParser } from "../../core/errors/IErrorParser";
import { IMessageRepository } from "../../core/persistence/IMessageRepository";
import { IMessageStatusReader, IMessageClaimer } from "../../core/services/contracts/IMessageServiceContract";
import { INonceManager } from "../../core/services/INonceManager";
import { ITransactionValidationService } from "../../core/services/ITransactionValidationService";
import {
  IMessageClaimingProcessor,
  MessageClaimingProcessorConfig,
} from "../../core/services/processors/IMessageClaimingProcessor";

export class MessageClaimingProcessor implements IMessageClaimingProcessor {
  constructor(
    private readonly messageServiceContract: IMessageStatusReader & IMessageClaimer,
    private readonly nonceManager: INonceManager,
    private readonly messageRepository: IMessageRepository,
    private readonly getNextMessageToClaim: () => Promise<Message | null>,
    private readonly transactionValidationService: ITransactionValidationService,
    private readonly errorParser: IErrorParser,
    private readonly config: MessageClaimingProcessorConfig,
    private readonly logger: ILogger,
  ) {}

  public async process(): Promise<void> {
    let nextMessageToClaim: Message | null = null;
    let nonce: number | null = null;

    try {
      nextMessageToClaim = await this.getNextMessageToClaim();

      if (!nextMessageToClaim) {
        this.logger.debug("No message to claim found.");
        return;
      }

      this.logger.info("Found message to claim.", { messageHash: nextMessageToClaim.messageHash });

      const messageStatus = await this.messageServiceContract.getMessageStatus({
        messageHash: nextMessageToClaim.messageHash,
        messageBlockNumber: nextMessageToClaim.sentBlockNumber,
      });

      if (messageStatus === OnChainMessageStatus.CLAIMED) {
        this.logger.info("Found already claimed message.", { messageHash: nextMessageToClaim.messageHash });

        nextMessageToClaim.edit({ status: MessageStatus.CLAIMED_SUCCESS });
        await this.messageRepository.updateMessage(nextMessageToClaim);
        return;
      }

      this.logger.debug("Evaluating transaction.", { messageHash: nextMessageToClaim.messageHash });

      const {
        hasZeroFee,
        isUnderPriced,
        isRateLimitExceeded,
        isForSponsorship,
        estimatedGasLimit,
        threshold,
        ...claimTxFees
      } = await this.transactionValidationService.evaluateTransaction(
        nextMessageToClaim,
        this.config.feeRecipientAddress,
        this.config.claimViaAddress,
      );

      // If isForSponsorship = true, then we ignore hasZeroFee and isUnderPriced
      if (!isForSponsorship && (await this.handleZeroFee(hasZeroFee, nextMessageToClaim))) return;
      if (await this.handleNonExecutable(nextMessageToClaim, estimatedGasLimit)) return;

      nextMessageToClaim.edit({ claimGasEstimationThreshold: threshold, isForSponsorship });
      await this.messageRepository.updateMessage(nextMessageToClaim);

      if (
        !isForSponsorship &&
        (await this.handleUnderpriced(nextMessageToClaim, isUnderPriced, estimatedGasLimit, claimTxFees.maxFeePerGas))
      )
        return;
      if (this.handleRateLimitExceeded(nextMessageToClaim, isRateLimitExceeded)) return;

      nonce = await this.nonceManager.acquireNonce();

      this.logger.debug("Executing claim transaction.", {
        messageHash: nextMessageToClaim.messageHash,
        nonce,
        gasLimit: estimatedGasLimit!.toString(),
      });

      await this.executeClaimTransaction(
        nextMessageToClaim,
        nonce,
        estimatedGasLimit!,
        claimTxFees.maxPriorityFeePerGas,
        claimTxFees.maxFeePerGas,
      );
      this.nonceManager.commitNonce(nonce);
    } catch (e) {
      if (nonce !== null) this.nonceManager.rollbackNonce(nonce);
      await this.handleProcessingError(e, nextMessageToClaim);
    }
  }

  private async executeClaimTransaction(
    message: Message,
    nonce: number,
    gasLimit: bigint,
    maxPriorityFeePerGas: bigint,
    maxFeePerGas: bigint,
  ): Promise<void> {
    const isRetry = message.status === MessageStatus.FEE_UNDERPRICED;

    // Submit transaction to chain before writing PENDING to DB, so we never expose
    // a PENDING row without a claimTxHash (avoids race with MessageClaimingPersister).
    const claimTxCreationDate = new Date();
    const tx = await this.messageServiceContract.claim(
      {
        ...message,
        feeRecipient: this.config.feeRecipientAddress,
        messageBlockNumber: message.sentBlockNumber,
      },
      {
        claimViaAddress: this.config.claimViaAddress,
        overrides: { nonce, gasLimit, maxPriorityFeePerGas, maxFeePerGas },
      },
    );

    // Single DB write: set PENDING with all tx details atomically
    message.edit({
      claimTxCreationDate,
      claimTxNonce: nonce,
      status: MessageStatus.PENDING,
      claimTxGasLimit: Number(tx.gasLimit),
      claimTxMaxFeePerGas: tx.maxFeePerGas ?? undefined,
      claimTxMaxPriorityFeePerGas: tx.maxPriorityFeePerGas ?? undefined,
      claimTxHash: tx.hash,
      ...(isRetry ? { claimNumberOfRetry: message.claimNumberOfRetry + 1, claimLastRetriedAt: new Date() } : {}),
    });
    await this.messageRepository.updateMessage(message);
  }

  private async handleZeroFee(hasZeroFee: boolean, message: Message): Promise<boolean> {
    if (hasZeroFee) {
      this.logger.warn("Found message with zero fee. This message will not be processed.", {
        messageHash: message.messageHash,
      });
      message.edit({ status: MessageStatus.ZERO_FEE });
      await this.messageRepository.updateMessage(message);
      return true;
    }
    return false;
  }

  private async handleNonExecutable(message: Message, estimatedGasLimit: bigint | null): Promise<boolean> {
    if (!estimatedGasLimit) {
      this.logger.warn("Estimated gas limit is higher than the max allowed gas limit for this message.", {
        messageHash: message.messageHash,
        messageInfo: message.toString(),
        estimatedGasLimit: estimatedGasLimit?.toString(),
        maxAllowedGasLimit: this.config.maxClaimGasLimit.toString(),
      });
      message.edit({ status: MessageStatus.NON_EXECUTABLE });
      await this.messageRepository.updateMessage(message);
      return true;
    }
    return false;
  }

  private async handleUnderpriced(
    message: Message,
    isUnderPriced: boolean,
    estimatedGasLimit: bigint | null,
    maxFeePerGas: bigint,
  ): Promise<boolean> {
    if (isUnderPriced) {
      if (message.status !== MessageStatus.FEE_UNDERPRICED) {
        this.logger.warn("Fee underpriced found in this message.", {
          messageHash: message.messageHash,
          messageInfo: message.toString(),
          transactionGasLimit: estimatedGasLimit?.toString(),
          maxFeePerGas: maxFeePerGas.toString(),
        });
        message.edit({ status: MessageStatus.FEE_UNDERPRICED });
        await this.messageRepository.updateMessage(message);
      } else {
        this.logger.warn("Message is underpriced, will retry later.", { messageHash: message.messageHash });
      }
      return true;
    }
    return false;
  }

  private handleRateLimitExceeded(message: Message, isRateLimitExceeded: boolean): boolean {
    if (isRateLimitExceeded) {
      this.logger.warn("Rate limit exceeded for this message. It will be reprocessed later.", {
        messageHash: message.messageHash,
      });
      return true;
    }
    return false;
  }

  private async handleProcessingError(e: unknown, message: Message | null): Promise<void> {
    const parsedError = this.errorParser.parse(e);

    if (!parsedError.retryable && message) {
      message.edit({ status: MessageStatus.NON_EXECUTABLE });
      await this.messageRepository.updateMessage(message);
    }

    this.logger.error("Error processing message claim.", {
      error: e,
      parsedError,
      ...(message ? { messageHash: message.messageHash } : {}),
    });
  }
}
