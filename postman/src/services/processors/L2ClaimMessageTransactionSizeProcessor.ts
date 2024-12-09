import {
  ContractTransactionResponse,
  ErrorDescription,
  Overrides,
  Signer,
  TransactionReceipt,
  TransactionResponse,
} from "ethers";
import { MessageStatus } from "../../core/enums";
import { ILogger } from "../../core/utils/logging/ILogger";
import { IMessageDBService } from "../../core/persistence/IMessageDBService";
import { IL2MessageServiceClient } from "../../core/clients/blockchain/linea/IL2MessageServiceClient";
import {
  IL2ClaimMessageTransactionSizeProcessor,
  L2ClaimMessageTransactionSizeProcessorConfig,
} from "../../core/services/processors/IL2ClaimMessageTransactionSizeProcessor";
import { IL2ClaimTransactionSizeCalculator } from "../../core/services/processors/IL2ClaimTransactionSizeCalculator";

export class L2ClaimMessageTransactionSizeProcessor implements IL2ClaimMessageTransactionSizeProcessor {
  /**
   * Constructs a new instance of the `L2ClaimMessageTransactionSizeProcessor`.
   *
   * @param {IMessageDBService} databaseService - The database service for interacting with message data.
   * @param {IL2MessageServiceClient} l2MessageServiceClient - The L2 message service client for estimating gas fees.
   * @param {IL2ClaimTransactionSizeCalculator} transactionSizeCalculator - The calculator for determining the transaction size.
   * @param {L2ClaimMessageTransactionSizeProcessorConfig} config - Configuration settings for the processor, including the direction and origin contract address.
   * @param {ILogger} logger - The logger for logging information and errors.
   */
  constructor(
    private readonly databaseService: IMessageDBService<ContractTransactionResponse>,
    private readonly l2MessageServiceClient: IL2MessageServiceClient<
      Overrides,
      TransactionReceipt,
      TransactionResponse,
      ContractTransactionResponse,
      Signer,
      ErrorDescription
    >,
    private readonly transactionSizeCalculator: IL2ClaimTransactionSizeCalculator,
    private readonly config: L2ClaimMessageTransactionSizeProcessorConfig,
    private readonly logger: ILogger,
  ) {}

  /**
   * Processes the transaction size and gas limit for L2 claim messages.
   * Fetches the first anchored message, calculates its transaction size and gas limit, updates the message status, and logs the information.
   *
   * @returns {Promise<void>} A promise that resolves when the processing is complete.
   */
  public async process(): Promise<void> {
    try {
      const messages = await this.databaseService.getNFirstMessagesByStatus(
        MessageStatus.ANCHORED,
        this.config.direction,
        1,
        this.config.originContractAddress,
      );

      if (messages.length === 0) {
        return;
      }

      const message = messages[0];

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

      await this.databaseService.updateMessage(message);

      this.logger.info(
        "Message transaction size and gas limit have been computed: messageHash=%s transactionSize=%s gasLimit=%s",
        message.messageHash,
        transactionSize,
        gasLimit,
      );
    } catch (e) {
      this.logger.error(e);
    }
  }
}
