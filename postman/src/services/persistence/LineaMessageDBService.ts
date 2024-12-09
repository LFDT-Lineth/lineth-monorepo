import {
  Block,
  ContractTransactionResponse,
  JsonRpcProvider,
  TransactionReceipt,
  TransactionRequest,
  TransactionResponse,
} from "ethers";
import { Direction } from "@consensys/linea-sdk";
import { Message } from "../../core/entities/Message";
import { MessageStatus } from "../../core/enums";
import { IMessageRepository } from "../../core/persistence/IMessageRepository";
import { ILineaProvider } from "../../core/clients/blockchain/linea/ILineaProvider";
import { BaseError } from "../../core/errors";
import { IMessageDBService } from "../../core/persistence/IMessageDBService";
import { MessageDBService } from "./MessageDBService";
import { MINIMUM_MARGIN } from "../../core/constants";

export class LineaMessageDBService extends MessageDBService implements IMessageDBService<ContractTransactionResponse> {
  /**
   * Creates an instance of `LineaMessageDBService`.
   *
   * @param {ILineaProvider} provider - The provider for interacting with the blockchain.
   * @param {IMessageRepository} messageRepository - The message repository for interacting with the message database.
   */
  constructor(
    private readonly provider: ILineaProvider<
      TransactionReceipt,
      Block,
      TransactionRequest,
      TransactionResponse,
      JsonRpcProvider
    >,
    messageRepository: IMessageRepository<ContractTransactionResponse>,
  ) {
    super(messageRepository);
  }

  /**
   * Retrieves the first N messages with status SENT and direction L1_TO_L2.
   *
   * @param {number} limit - The maximum number of messages to retrieve.
   * @param {string} contractAddress - The address of the contract to filter messages by.
   * @returns {Promise<Message[]>} A promise that resolves to an array of messages.
   */
  public async getNFirstMessagesSent(limit: number, contractAddress: string): Promise<Message[]> {
    return this.messageRepository.getNFirstMessagesByStatus(
      MessageStatus.SENT,
      Direction.L1_TO_L2,
      limit,
      contractAddress,
    );
  }

  /**
   * Retrieves the first message to claim on L2.
   *
   * @param {string} contractAddress - The address of the contract to filter messages by.
   * @param {number} _gasEstimationMargin - The margin to apply to gas estimation.
   * @param {number} maxRetry - The maximum number of retries for claiming the message.
   * @param {number} retryDelay - The delay between retries in milliseconds.
   * @returns {Promise<Message | null>} A promise that resolves to the message to claim, or null if no message is found.
   */
  public async getMessageToClaim(
    contractAddress: string,
    _gasEstimationMargin: number,
    maxRetry: number,
    retryDelay: number,
  ): Promise<Message | null> {
    const feeEstimationOptions = await this.getClaimDBQueryFeeOptions();
    return this.messageRepository.getFirstMessageToClaimOnL2(
      Direction.L1_TO_L2,
      contractAddress,
      [MessageStatus.TRANSACTION_SIZE_COMPUTED, MessageStatus.FEE_UNDERPRICED],
      maxRetry,
      retryDelay,
      feeEstimationOptions,
    );
  }

  /**
   * Retrieves fee estimation options for querying the database.
   *
   * @private
   * @returns {Promise<{ minimumMargin: number; extraDataVariableCost: number; extraDataFixedCost: number }>} A promise that resolves to an object containing fee estimation options.
   * @throws {BaseError} If no extra data is available.
   */
  private async getClaimDBQueryFeeOptions(): Promise<{
    minimumMargin: number;
    extraDataVariableCost: number;
    extraDataFixedCost: number;
  }> {
    const minimumMargin = MINIMUM_MARGIN;
    const blockNumber = await this.provider.getBlockNumber();
    const extraData = await this.provider.getBlockExtraData(blockNumber);

    if (!extraData) {
      throw new BaseError("no extra data.");
    }
    return {
      minimumMargin,
      extraDataVariableCost: extraData.variableCost,
      extraDataFixedCost: extraData.fixedCost,
    };
  }
}
