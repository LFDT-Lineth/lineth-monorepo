import { ILogger } from "@consensys/linea-shared-utils";

import { ITransactionProvider } from "../../core/clients/blockchain/IProvider";
import { Message } from "../../core/entities/Message";
import { MessageStatus } from "../../core/enums";
import { IMessageRepository } from "../../core/persistence/IMessageRepository";
import {
  IMessageClaimingPersister,
  MessageClaimingPersisterConfig,
} from "../../core/services/processors/IMessageClaimingPersister";
import { IReceiptStatusResolver } from "../../core/services/processors/IReceiptStatusResolver";
import { ITransactionLifecycleManager } from "../../core/services/processors/ITransactionLifecycleManager";

export class MessageClaimingPersister implements IMessageClaimingPersister {
  constructor(
    private readonly messageRepository: IMessageRepository,
    private readonly provider: ITransactionProvider,
    private readonly transactionLifecycleManager: ITransactionLifecycleManager,
    private readonly receiptStatusResolver: IReceiptStatusResolver,
    private readonly config: MessageClaimingPersisterConfig,
    private readonly logger: ILogger,
  ) {}

  public async process(): Promise<void> {
    let firstPendingMessage: Message | null = null;
    try {
      firstPendingMessage = await this.messageRepository.getFirstPendingMessage(this.config.direction);
      if (!firstPendingMessage) {
        this.logger.debug("No pending message status to update.");
        return;
      }

      if (!firstPendingMessage.claimTxHash) {
        this.logger.warn("Found pending message without claim tx hash, resetting to allow retry.", {
          messageHash: firstPendingMessage.messageHash,
        });
        firstPendingMessage.edit({ status: MessageStatus.SENT });
        await this.messageRepository.updateMessage(firstPendingMessage);
        return;
      }

      this.logger.debug("Checking pending message.", {
        messageHash: firstPendingMessage.messageHash,
        claimTxHash: firstPendingMessage.claimTxHash,
      });

      const receipt = await this.provider.getTransactionReceipt(firstPendingMessage.claimTxHash);
      if (receipt) {
        await this.receiptStatusResolver.resolveReceiptStatus(firstPendingMessage, receipt, new Date());
        return;
      }

      if (!this.isMessageExceededSubmissionTimeout(firstPendingMessage)) return;

      if (firstPendingMessage.claimCycleCount >= this.config.maxCycles) {
        this.logger.error("Max retry cycles exceeded. Manual intervention is needed.", {
          messageHash: firstPendingMessage.messageHash,
          claimCycleCount: firstPendingMessage.claimCycleCount,
        });
        firstPendingMessage.edit({ status: MessageStatus.NEEDS_MANUAL_INTERVENTION });
        await this.messageRepository.updateMessage(firstPendingMessage);
        return;
      }

      this.logger.warn("Retrying to claim message.", { messageHash: firstPendingMessage.messageHash });

      if (firstPendingMessage.claimNumberOfRetry >= this.config.maxBumpsPerCycle) {
        const cancelReceipt = await this.transactionLifecycleManager.cancelAndResetMessage(firstPendingMessage);
        if (cancelReceipt) {
          await this.receiptStatusResolver.resolveReceiptStatus(firstPendingMessage, cancelReceipt, new Date());
        }
        return;
      }

      const retryReceipt = await this.transactionLifecycleManager.retryWithBump(firstPendingMessage);
      if (!retryReceipt) return;

      this.logger.warn("Retried claim message transaction succeed.", {
        messageHash: firstPendingMessage.messageHash,
        transactionHash: retryReceipt.hash,
      });
      await this.receiptStatusResolver.resolveReceiptStatus(firstPendingMessage, retryReceipt, new Date());
    } catch (e) {
      this.logger.error("Error processing pending message.", {
        error: e,
        ...(firstPendingMessage ? { messageHash: firstPendingMessage.messageHash } : {}),
      });
    }
  }

  private isMessageExceededSubmissionTimeout(message: Message): boolean {
    return (
      !!message.updatedAt && new Date().getTime() - message.updatedAt.getTime() > this.config.messageSubmissionTimeout
    );
  }
}
