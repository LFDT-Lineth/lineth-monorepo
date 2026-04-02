import { ILogger } from "@consensys/linea-shared-utils";

import { IBlockProvider } from "../../core/clients/blockchain/IProvider";
import { Message } from "../../core/entities/Message";
import { Direction, MessageStatus } from "../../core/enums";
import { ISponsorshipMetricsUpdater, ITransactionMetricsUpdater } from "../../core/metrics";
import { IMessageRepository } from "../../core/persistence/IMessageRepository";
import { IRateLimitChecker } from "../../core/services/contracts/IMessageServiceContract";
import {
  IReceiptStatusResolver,
  ReceiptStatusResolverConfig,
} from "../../core/services/processors/IReceiptStatusResolver";
import { TransactionReceipt } from "../../core/types";

export class ReceiptStatusResolver implements IReceiptStatusResolver {
  constructor(
    private readonly messageRepository: IMessageRepository,
    private readonly messageServiceContract: IRateLimitChecker,
    private readonly provider: IBlockProvider,
    private readonly sponsorshipMetricsUpdater: ISponsorshipMetricsUpdater,
    private readonly transactionMetricsUpdater: ITransactionMetricsUpdater,
    private readonly config: ReceiptStatusResolverConfig,
    private readonly logger: ILogger,
  ) {}

  public async resolveReceiptStatus(
    message: Message,
    receipt: TransactionReceipt,
    receiptReceivedAt: Date,
  ): Promise<void> {
    const timingMetrics = await this.computeTimingMetrics(message, receipt, receiptReceivedAt);

    if (receipt.status === "reverted") {
      await this.handleRevertedReceipt(message, receipt, timingMetrics);
      return;
    }

    message.edit({ status: MessageStatus.CLAIMED_SUCCESS });
    await this.messageRepository.updateMessage(message);

    if (message.isForSponsorship) {
      await this.sponsorshipMetricsUpdater.incrementSponsorshipFeePaid(
        receipt.gasPrice * receipt.gasUsed,
        message.direction,
      );
    }

    this.logger.info("Message has been SUCCESSFULLY claimed.", {
      messageHash: message.messageHash,
      transactionHash: receipt.hash,
      ...timingMetrics,
    });
  }

  private async computeTimingMetrics(
    message: Message,
    receipt: TransactionReceipt,
    receiptReceivedAt: Date,
  ): Promise<Record<string, number>> {
    if (this.config.direction !== Direction.L1_TO_L2 || !message.claimTxCreationDate) {
      return {};
    }

    const block = await this.provider.getBlock(receipt.blockNumber);
    if (!block) {
      return {};
    }

    const processingTimeInSeconds = Math.max(0, block.timestamp - message.claimTxCreationDate.getTime() / 1_000);
    const infuraConfirmationTimeInSeconds = Math.max(
      0,
      (receiptReceivedAt.getTime() - message.claimTxCreationDate.getTime()) / 1_000,
    );

    this.transactionMetricsUpdater.addTransactionProcessingTime(this.config.direction, processingTimeInSeconds);
    this.transactionMetricsUpdater.addTransactionInfuraConfirmationTime(
      this.config.direction,
      infuraConfirmationTimeInSeconds,
    );

    return { processingTimeInSeconds, infuraConfirmationTimeInSeconds };
  }

  private async handleRevertedReceipt(
    message: Message,
    receipt: TransactionReceipt,
    timingMetrics: Record<string, number>,
  ): Promise<void> {
    // Pass blockNumber so the simulation replays at the original block; otherwise a
    // rate-limit that was active then but has since reset would not be detected.
    const isRateLimitExceeded = await this.messageServiceContract.isRateLimitExceededError(
      receipt.hash,
      BigInt(receipt.blockNumber),
    );

    if (isRateLimitExceeded) {
      message.edit({ status: MessageStatus.SENT });
      await this.messageRepository.updateMessage(message);

      this.logger.warn(
        "Claim transaction has been reverted with RateLimitExceeded error. Claiming will be retry later.",
        { messageHash: message.messageHash, transactionHash: receipt.hash },
      );
      return;
    }

    message.edit({ status: MessageStatus.CLAIMED_REVERTED });
    await this.messageRepository.updateMessage(message);
    this.logger.warn("Message claim transaction has been REVERTED.", {
      messageHash: message.messageHash,
      transactionHash: receipt.hash,
      ...timingMetrics,
    });
  }
}
