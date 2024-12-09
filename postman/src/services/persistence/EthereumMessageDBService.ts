import { ContractTransactionResponse, TransactionRequest } from "ethers";
import { Direction } from "@consensys/linea-sdk";
import { Message } from "../../core/entities/Message";
import { MessageStatus } from "../../core/enums";
import { IMessageRepository } from "../../core/persistence/IMessageRepository";
import { IMessageDBService } from "../../core/persistence/IMessageDBService";
import { IGasProvider } from "../../core/clients/blockchain/IGasProvider";
import { MessageDBService } from "./MessageDBService";

export class EthereumMessageDBService
  extends MessageDBService
  implements IMessageDBService<ContractTransactionResponse>
{
  /**
   * Creates an instance of `EthereumMessageDBService`.
   *
   * @param {IGasProvider} gasProvider - The gas provider for fetching gas fee estimates.
   * @param {IMessageRepository} messageRepository - The message repository for interacting with the message database.
   */
  constructor(
    private readonly gasProvider: IGasProvider<TransactionRequest>,
    messageRepository: IMessageRepository<ContractTransactionResponse>,
  ) {
    super(messageRepository);
  }

  /**
   * Retrieves the first N messages with status SENT and direction L2_TO_L1.
   *
   * @param {number} limit - The maximum number of messages to retrieve.
   * @param {string} contractAddress - The address of the contract to filter messages by.
   * @returns {Promise<Message[]>} A promise that resolves to an array of messages.
   */
  public async getNFirstMessagesSent(limit: number, contractAddress: string): Promise<Message[]> {
    return this.messageRepository.getNFirstMessagesByStatus(
      MessageStatus.SENT,
      Direction.L2_TO_L1,
      limit,
      contractAddress,
    );
  }

  /**
   * Retrieves the first message to claim on L1.
   *
   * @param {string} contractAddress - The address of the contract to filter messages by.
   * @param {number} gasEstimationMargin - The margin to apply to gas estimation.
   * @param {number} maxRetry - The maximum number of retries for claiming the message.
   * @param {number} retryDelay - The delay between retries in milliseconds.
   * @returns {Promise<Message | null>} A promise that resolves to the message to claim, or null if no message is found.
   */
  public async getMessageToClaim(
    contractAddress: string,
    gasEstimationMargin: number,
    maxRetry: number,
    retryDelay: number,
  ): Promise<Message | null> {
    const { maxFeePerGas } = await this.gasProvider.getGasFees();
    return this.messageRepository.getFirstMessageToClaimOnL1(
      Direction.L2_TO_L1,
      contractAddress,
      maxFeePerGas,
      gasEstimationMargin,
      maxRetry,
      retryDelay,
    );
  }
}
