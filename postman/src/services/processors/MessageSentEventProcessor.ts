import {
  Block,
  ContractTransactionResponse,
  JsonRpcProvider,
  TransactionReceipt,
  TransactionRequest,
  TransactionResponse,
} from "ethers";
import { serialize, isEmptyBytes } from "@consensys/linea-sdk";
import { ILineaRollupLogClient } from "../../core/clients/blockchain/ethereum/ILineaRollupLogClient";
import { IProvider } from "../../core/clients/blockchain/IProvider";
import { MessageFactory } from "../../core/entities/MessageFactory";
import { ILogger } from "../../core/utils/logging/ILogger";
import { MessageStatus } from "../../core/enums";
import { IL2MessageServiceLogClient } from "../../core/clients/blockchain/linea/IL2MessageServiceLogClient";
import {
  IMessageSentEventProcessor,
  MessageSentEventProcessorConfig,
} from "../../core/services/processors/IMessageSentEventProcessor";
import { IMessageDBService } from "../../core/persistence/IMessageDBService";

export class MessageSentEventProcessor implements IMessageSentEventProcessor {
  private readonly maxBlocksToFetchLogs: number;

  /**
   * Initializes a new instance of the `MessageSentEventProcessor`.
   *
   * @param {IMessageDBService} databaseService - An instance of a class implementing the `IMessageDBService` interface, used for storing and retrieving message data.
   * @param {ILineaRollupLogClient | IL2MessageServiceLogClient} logClient - An instance of a class implementing the `ILineaRollupLogClient` or the `IL2MessageServiceLogClient` interface for fetching message sent events from the blockchain.
   * @param {IProvider} provider - An instance of a class implementing the `IProvider` interface, used to query blockchain data.
   * @param {MessageSentEventProcessorConfig} config - Configuration for network-specific settings, including listener parameters and feature flags.
   * @param {ILogger} logger - An instance of a class implementing the `ILogger` interface, used for logging messages.
   */
  constructor(
    private readonly databaseService: IMessageDBService<ContractTransactionResponse>,
    private readonly logClient: ILineaRollupLogClient | IL2MessageServiceLogClient,
    private readonly provider: IProvider<
      TransactionReceipt,
      Block,
      TransactionRequest,
      TransactionResponse,
      JsonRpcProvider
    >,
    private readonly config: MessageSentEventProcessorConfig,
    private readonly logger: ILogger,
  ) {
    this.maxBlocksToFetchLogs = Math.max(config.maxBlocksToFetchLogs, 0);
  }

  /**
   * Calculates the starting block number for fetching events, ensuring it is within the valid range.
   *
   * @param {number} fromBlockNumber - The proposed starting block number.
   * @param {number} toBlockNumber - The ending block number for the query range.
   * @returns {number} The adjusted starting block number.
   */
  private calculateFromBlockNumber(fromBlockNumber: number, toBlockNumber: number): number {
    if (fromBlockNumber > toBlockNumber) {
      return toBlockNumber;
    }
    return Math.max(fromBlockNumber, 0);
  }

  /**
   * Fetches `MessageSent` events from the blockchain within a specified block range and stores them in the database.
   *
   * @param {number} fromBlock - The starting block number for fetching events.
   * @param {number} fromBlockLogIndex - The log index within the starting block to begin processing events from.
   * @returns {Promise<{ nextFromBlock: number; nextFromBlockLogIndex: number }>} The block number and log index to start fetching events from in the next iteration.
   */
  public async process(
    fromBlock: number,
    fromBlockLogIndex: number,
  ): Promise<{ nextFromBlock: number; nextFromBlockLogIndex: number }> {
    const latestBlockNumber = Math.max((await this.provider.getBlockNumber()) - this.config.blockConfirmation, 0);
    const toBlock = Math.min(latestBlockNumber, fromBlock + this.maxBlocksToFetchLogs);

    fromBlock = this.calculateFromBlockNumber(fromBlock, toBlock);

    this.logger.info("Getting events fromBlock=%s toBlock=%s", fromBlock, toBlock);

    const events = await this.logClient.getMessageSentEvents({
      fromBlock,
      toBlock,
      fromBlockLogIndex,
    });

    this.logger.info("Number of fetched MessageSent events: %s", events.length);

    for (const event of events) {
      const shouldBeProcessed = this.shouldProcessMessage(event.calldata, event.messageHash);
      const messageStatusToInsert = shouldBeProcessed ? MessageStatus.SENT : MessageStatus.EXCLUDED;

      const message = MessageFactory.createMessage({
        ...event,
        sentBlockNumber: event.blockNumber,
        direction: this.config.direction,
        status: messageStatusToInsert,
        claimNumberOfRetry: 0,
      });

      await this.databaseService.insertMessage(message);
    }
    this.logger.info(`Messages hashes found: messageHashes=%s`, serialize(events.map((event) => event.messageHash)));

    return { nextFromBlock: toBlock + 1, nextFromBlockLogIndex: 0 };
  }

  /**
   * Determines whether a message should be processed based on its calldata and the configuration.
   *
   * @param {string} messageCalldata - The calldata of the message.
   * @param {string} messageHash - The hash of the message.
   * @returns {boolean} `true` if the message should be processed, `false` otherwise.
   */
  private shouldProcessMessage(messageCalldata: string, messageHash: string): boolean {
    if (isEmptyBytes(messageCalldata)) {
      if (this.config.isEOAEnabled) {
        return true;
      }
    } else {
      if (this.config.isCalldataEnabled) {
        return true;
      }
    }

    this.logger.debug(
      "Message has been excluded because target address is not an EOA or calldata is not empty: messageHash=%s",
      messageHash,
    );
    return false;
  }
}
