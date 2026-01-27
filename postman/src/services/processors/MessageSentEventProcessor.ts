import { serialize, isEmptyBytes, MessageSent } from "@consensys/linea-sdk";
import { ILogger } from "@consensys/linea-shared-utils";
import {
  Block,
  ContractTransactionResponse,
  dataSlice,
  Interface,
  JsonRpcProvider,
  TransactionReceipt,
  TransactionRequest,
  TransactionResponse,
} from "ethers";
import { compileExpression, useDotAccessOperator } from "filtrex";

import { ILineaRollupLogClient } from "../../core/clients/blockchain/ethereum/ILineaRollupLogClient";
import { IProvider } from "../../core/clients/blockchain/IProvider";
import { IL2MessageServiceLogClient } from "../../core/clients/blockchain/linea/IL2MessageServiceLogClient";
import { MessageFactory } from "../../core/entities/MessageFactory";
import { MessageStatus } from "../../core/enums";
import { IMessageDBService } from "../../core/persistence/IMessageDBService";
import {
  IMessageSentEventProcessor,
  MessageSentEventProcessorConfig,
} from "../../core/services/processors/IMessageSentEventProcessor";

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
    protected readonly config: MessageSentEventProcessorConfig,
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
      filters: {
        from: this.config.eventFilters?.fromAddressFilter,
        to: this.config.eventFilters?.toAddressFilter,
      },
      fromBlock,
      toBlock,
      fromBlockLogIndex,
    });

    this.logger.info("Number of fetched MessageSent events: %s", events.length);

    for (const event of events) {
      const shouldBeProcessed = this.shouldProcessMessage(
        event,
        event.messageHash,
        this.config.eventFilters?.calldataFilter,
      );
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
   * @param {string} event - The message event.
   * @param {string} messageHash - The hash of the message.
   * @returns {boolean} `true` if the message should be processed, `false` otherwise.
   */
  protected shouldProcessMessage(
    event: MessageSent,
    messageHash: string,
    filters?: {
      criteriaExpression: string;
      calldataFunctionInterface: string;
    },
  ): boolean {
    const hasEmptyCalldata = isEmptyBytes(event.calldata);
    let basicProcess = false;

    if (hasEmptyCalldata) {
      basicProcess = this.config.isEOAEnabled;
    } else {
      basicProcess = this.config.isCalldataEnabled;
    }

    if (!basicProcess) {
      this.logger.debug(
        "Message has been excluded because target address is not an EOA or calldata is not empty: messageHash=%s",
        messageHash,
      );
      return false;
    }

    if (!hasEmptyCalldata && this.config.isCalldataEnabled && !this.isMessageMatchingCriteria(event, filters)) {
      return false;
    }

    return true;
  }

  private isMessageMatchingCriteria(
    event: MessageSent,
    filters?: { criteriaExpression: string; calldataFunctionInterface: string },
  ) {
    if (!filters) {
      return true;
    }

    const iface = new Interface([filters.calldataFunctionInterface]);
    const decodedCalldata = iface.decodeFunctionData(filters.calldataFunctionInterface, event.calldata);

    const context = {
      calldata: {
        funcSignature: dataSlice(event.calldata, 0, 4),
        ...this.convertBigInts(decodedCalldata.toObject(true)),
      },
    };

    const passesFilter = this.evaluateExpression(filters.criteriaExpression, context);

    if (!passesFilter) {
      this.logger.debug(
        "Message has been excluded because it does not match the criteria: criteria=%s messageHash=%s transactionHash=%s",
        filters.criteriaExpression,
        event.messageHash,
        event.transactionHash,
      );
      return false;
    }

    return true;
  }

  private evaluateExpression(expression: string, context: unknown): boolean {
    try {
      const compiledFilter = compileExpression(expression, { customProp: useDotAccessOperator });
      const passesFilter = compiledFilter(context);
      return passesFilter === true;
    } catch {
      return false;
    }
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private convertBigInts(data: any): any {
    if (typeof data === "bigint") {
      return Number(data);
    }

    if (Array.isArray(data)) {
      return data.map((item) => this.convertBigInts(item));
    }

    if (data !== null && typeof data === "object") {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const result: Record<string, any> = {};
      for (const key in data) {
        if (Object.prototype.hasOwnProperty.call(data, key)) {
          result[key] = this.convertBigInts(data[key]);
        }
      }
      return result;
    }

    return data;
  }
}
