import {
  Block,
  ContractTransactionResponse,
  JsonRpcProvider,
  TransactionReceipt,
  TransactionRequest,
  TransactionResponse,
} from "ethers";
import { Direction, wait } from "@consensys/linea-sdk";
import { DEFAULT_INITIAL_FROM_BLOCK } from "../../core/constants";
import { IPostmanLogger } from "../../utils/IPostmanLogger";
import { IMessageSentEventProcessor } from "../../core/services/processors/IMessageSentEventProcessor";
import { Message } from "../../core/entities/Message";
import { DatabaseAccessError } from "../../core/errors/DatabaseErrors";
import { IProvider } from "../../core/clients/blockchain/IProvider";
import { IPoller } from "../../core/services/pollers/IPoller";
import { IMessageDBService } from "../../core/persistence/IMessageDBService";

type MessageSentEventPollerConfig = {
  direction: Direction;
  pollingInterval: number;
  initialFromBlock: number;
  originContractAddress: string;
};

export class MessageSentEventPoller implements IPoller {
  private isPolling = false;

  /**
   * Constructs a new instance of the `MessageSentEventPoller`.
   *
   * @param {IMessageSentEventProcessor} eventProcessor - An instance of a class implementing the `IMessageSentEventProcessor` interface, responsible for processing message sent events.
   * @param {IProvider} provider - An instance of a class implementing the `IProvider` interface, used to query blockchain data.
   * @param {IMessageDBService} databaseService - An instance of a class implementing the `IMessageDBService` interface, used for storing and retrieving message data.
   * @param {MessageSentEventPollerConfig} config - Configuration settings for the poller, including the direction of message flow, the polling interval, and the initial block number to start listening from.
   * @param {IPostmanLogger} logger - An instance of a class implementing the `IPostmanLogger` interface, used for logging messages related to the polling process.
   */
  constructor(
    private readonly eventProcessor: IMessageSentEventProcessor,
    private readonly provider: IProvider<
      TransactionReceipt,
      Block,
      TransactionRequest,
      TransactionResponse,
      JsonRpcProvider
    >,
    private readonly databaseService: IMessageDBService<ContractTransactionResponse>,
    private readonly config: MessageSentEventPollerConfig,
    private readonly logger: IPostmanLogger,
  ) {}

  /**
   * Starts the polling process, initiating the continuous listening and processing of message sent events.
   * Logs a warning if the poller is already running.
   *
   * @returns {Promise<void>} A promise that resolves when the poller is started.
   */
  public async start(): Promise<void> {
    if (this.isPolling) {
      this.logger.warn("%s has already started.", this.logger.name);
      return;
    }

    this.logger.info("Starting %s %s...", this.config.direction, this.logger.name);

    this.isPolling = true;
    this.startProcessingEvents();
  }

  /**
   * Stops the polling process, halting any further listening and processing of message sent events.
   * Logs information about the stopping process.
   */
  public stop() {
    this.logger.info("Stopping %s %s...", this.config.direction, this.logger.name);
    this.isPolling = false;
    this.logger.info("%s %s stopped.", this.config.direction, this.logger.name);
  }

  /**
   * Initiates the event processing loop, fetching and processing message sent events starting from a determined block number.
   *
   * In case of errors during the initial block number retrieval or event processing, it attempts to restart the process after waiting for the specified polling interval.
   */
  private async startProcessingEvents(): Promise<void> {
    try {
      const { fromBlock, fromBlockLogIndex } = await this.getInitialFromBlock();
      this.processEvents(fromBlock, fromBlockLogIndex);
    } catch (e) {
      this.logger.error(e);
      await wait(this.config.pollingInterval);
      this.startProcessingEvents();
    }
  }

  /**
   * Processes message sent events starting from a specific block number and log index.
   *
   * This method continuously fetches and processes events, updating the starting point for the next fetch based on the processed events. In case of database access errors, it attempts to recover by restarting from the last successfully processed block number and log index.
   *
   * @param {number} fromBlock - The block number to start fetching events from.
   * @param {number} fromBlockLogIndex - The log index within the starting block to begin processing events from.
   */
  private async processEvents(fromBlock: number, fromBlockLogIndex: number): Promise<void> {
    if (!this.isPolling) return;

    try {
      const { nextFromBlock, nextFromBlockLogIndex } = await this.eventProcessor.process(fromBlock, fromBlockLogIndex);
      fromBlock = nextFromBlock;
      fromBlockLogIndex = nextFromBlockLogIndex;
    } catch (e) {
      if (e instanceof DatabaseAccessError) {
        fromBlock = (e.rejectedMessage as Message & { logIndex: number }).sentBlockNumber;
        fromBlockLogIndex = (e.rejectedMessage as Message & { logIndex: number }).logIndex;
        this.logger.warn(
          "Something went wrong with database access. Restarting fromBlockNum=%s and fromLogIndex=%s and errorMessage=%s",
          fromBlock,
          fromBlockLogIndex,
          e.message,
        );
      } else {
        this.logger.warnOrError(e);
      }
    } finally {
      await wait(this.config.pollingInterval);
      this.processEvents(fromBlock, fromBlockLogIndex);
    }
  }

  /**
   * Determines the initial block number and log index to start fetching message sent events from.
   *
   * This method considers the latest message sent block number, the configured initial block number, and the current block number to determine the most appropriate starting point for event processing.
   *
   * @returns {Promise<{ fromBlock: number; fromBlockLogIndex: number }>} An object containing the determined starting block number and log index.
   */
  private async getInitialFromBlock(): Promise<{ fromBlock: number; fromBlockLogIndex: number }> {
    let fromBlock = await this.provider.getBlockNumber();

    const fromBlockLogIndex = 0;

    const latestMessageSentBlockNumber = await this.getLatestMessageSentBlockNumber(this.config.direction);

    if (latestMessageSentBlockNumber) {
      fromBlock = latestMessageSentBlockNumber;
    }

    if (this.config.initialFromBlock > DEFAULT_INITIAL_FROM_BLOCK) {
      fromBlock = this.config.initialFromBlock;
    }

    return { fromBlock, fromBlockLogIndex };
  }

  /**
   * Retrieves the block number of the latest message sent event processed by the application, based on the specified direction and contract address.
   *
   * @param {Direction} direction - The direction of message flow to consider when retrieving the latest message sent event.
   * @returns {Promise<number | null>} The block number of the latest message sent event, or null if no such event has been processed.
   */
  private async getLatestMessageSentBlockNumber(direction: Direction): Promise<number | null> {
    const lastMessageSent = await this.databaseService.getLatestMessageSent(
      direction,
      this.config.originContractAddress,
    );

    if (!lastMessageSent) {
      return null;
    }

    return lastMessageSent.sentBlockNumber;
  }
}
