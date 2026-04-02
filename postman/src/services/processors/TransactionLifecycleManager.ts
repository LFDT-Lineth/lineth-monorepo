import { ILogger } from "@consensys/linea-shared-utils";

import { ITransactionProvider } from "../../core/clients/blockchain/IProvider";
import { Message } from "../../core/entities/Message";
import { OnChainMessageStatus, MessageStatus } from "../../core/enums";
import { IMessageRepository } from "../../core/persistence/IMessageRepository";
import { IMessageStatusReader } from "../../core/services/contracts/IMessageServiceContract";
import { IReceiptPoller } from "../../core/services/IReceiptPoller";
import { ITransactionRetrier } from "../../core/services/ITransactionRetrier";
import {
  ITransactionLifecycleManager,
  TransactionLifecycleConfig,
} from "../../core/services/processors/ITransactionLifecycleManager";
import { TransactionReceipt } from "../../core/types";

export class TransactionLifecycleManager implements ITransactionLifecycleManager {
  constructor(
    private readonly messageServiceContract: IMessageStatusReader,
    private readonly provider: ITransactionProvider,
    private readonly transactionRetrier: ITransactionRetrier,
    private readonly receiptPoller: IReceiptPoller,
    private readonly messageRepository: IMessageRepository,
    private readonly config: TransactionLifecycleConfig,
    private readonly logger: ILogger,
  ) {}

  public async retryWithBump(message: Message): Promise<TransactionReceipt | null> {
    try {
      const messageStatus = await this.messageServiceContract.getMessageStatus({
        messageHash: message.messageHash,
        messageBlockNumber: message.sentBlockNumber,
      });

      if (messageStatus === OnChainMessageStatus.CLAIMED) {
        const receipt = await this.provider.getTransactionReceipt(message.claimTxHash!);
        if (!receipt) {
          this.logger.warn("Message was claimed on-chain but transaction receipt is not available yet.", {
            messageHash: message.messageHash,
            transactionHash: message.claimTxHash,
          });
        }
        return receipt;
      }

      const attempt = message.claimNumberOfRetry + 1;
      this.logger.warn("Bumping fee for claim transaction.", {
        attempt: attempt.toString(),
        messageHash: message.messageHash,
      });

      const retryCreationDate = new Date();
      const tx = await this.transactionRetrier.retryWithHigherFee(message.claimTxHash!, attempt);

      message.edit({
        claimTxCreationDate: retryCreationDate,
        claimTxGasLimit: Number(tx.gasLimit),
        claimTxMaxFeePerGas: tx.maxFeePerGas ?? undefined,
        claimTxMaxPriorityFeePerGas: tx.maxPriorityFeePerGas ?? undefined,
        claimTxHash: tx.hash,
        claimNumberOfRetry: attempt,
        claimLastRetriedAt: new Date(),
        claimTxNonce: tx.nonce,
      });
      try {
        await this.messageRepository.updateMessage(message);
      } catch (dbError) {
        // The replacement tx is already on-chain — we must still poll for its receipt
        // so the persister can recover the final state. Losing track of the new hash
        // here would cause the next cycle to look up the superseded old hash, spin
        // indefinitely, and never reach cancelAndResetMessage.
        this.logger.error("DB update failed after fee bump; polling new tx to avoid losing it.", {
          messageHash: message.messageHash,
          newTxHash: tx.hash,
          error: dbError,
        });
      }

      return await this.receiptPoller.poll(
        tx.hash,
        this.config.receiptPollingTimeout,
        this.config.receiptPollingInterval,
      );
    } catch (e) {
      this.logger.error("Failed to retry with bumped fee.", { error: e, messageHash: message.messageHash });
      return null;
    }
  }

  public async cancelAndResetMessage(message: Message): Promise<TransactionReceipt | null> {
    try {
      const messageStatus = await this.messageServiceContract.getMessageStatus({
        messageHash: message.messageHash,
        messageBlockNumber: message.sentBlockNumber,
      });

      if (messageStatus === OnChainMessageStatus.CLAIMED) {
        const receipt = await this.provider.getTransactionReceipt(message.claimTxHash!);
        if (receipt) {
          return receipt;
        }
        this.logger.warn("Message claimed on-chain but receipt not available, will retry later.", {
          messageHash: message.messageHash,
        });
        return null;
      }

      this.logger.warn("Max fee bumps exhausted, cancelling stuck transaction and resetting message.", {
        messageHash: message.messageHash,
        claimTxNonce: message.claimTxNonce,
        claimCycleCount: message.claimCycleCount,
      });

      if (message.claimTxNonce !== undefined) {
        const stuckFees =
          message.claimTxMaxFeePerGas && message.claimTxMaxPriorityFeePerGas
            ? { maxFeePerGas: message.claimTxMaxFeePerGas, maxPriorityFeePerGas: message.claimTxMaxPriorityFeePerGas }
            : undefined;
        const cancelTxHash = await this.transactionRetrier.cancelTransaction(message.claimTxNonce, stuckFees);
        // Wait for the cancel tx to be mined before resetting, otherwise the anchoring
        // processor can re-queue the message while the cancel is still in flight.
        await this.receiptPoller.poll(
          cancelTxHash,
          this.config.receiptPollingTimeout,
          this.config.receiptPollingInterval,
        );
      }

      // edit() skips undefined values, so clear these directly before calling edit()
      message.claimTxHash = undefined;
      message.claimTxNonce = undefined;
      message.edit({
        status: MessageStatus.SENT,
        claimNumberOfRetry: 0,
        claimCycleCount: message.claimCycleCount + 1,
        claimLastRetriedAt: new Date(),
      });
      await this.messageRepository.updateMessage(message);

      this.logger.warn("Message reset to SENT for re-claiming with fresh nonce.", {
        messageHash: message.messageHash,
        newCycleCount: message.claimCycleCount,
      });

      return null;
    } catch (e) {
      this.logger.error("Failed to cancel and reset message.", {
        messageHash: message.messageHash,
        error: e,
      });
      return null;
    }
  }
}
