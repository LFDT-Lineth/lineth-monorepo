import { ILogger } from "@consensys/linea-shared-utils";
import { compileExpression, useDotAccessOperator } from "filtrex";

import { IMessageSentEventLogClient } from "../../core/clients/blockchain/ILogClient";
import { IBlockProvider } from "../../core/clients/blockchain/IProvider";
import { Message } from "../../core/entities/Message";
import { MessageStatus } from "../../core/enums";
import { IMessageRepository } from "../../core/persistence/IMessageRepository";
import { ICalldataDecoder } from "../../core/services/ICalldataDecoder";
import {
  IMessageSentEventProcessor,
  MessageSentEventProcessorConfig,
} from "../../core/services/processors/IMessageSentEventProcessor";
import { MessageSent } from "../../core/types";
import { isEmptyBytes, serialize } from "../../core/utils/shared";

export class MessageSentEventProcessor implements IMessageSentEventProcessor {
  private readonly maxBlocksToFetchLogs: number;

  /**
   * Initializes a new instance of the `MessageSentEventProcessor`.
   *
   * @param {IMessageRepository} messageRepository - Used for storing and retrieving message data.
   * @param {IMessageSentEventLogClient} logClient - For fetching message sent events from the blockchain.
   * @param {IProvider} provider - Used to query blockchain data.
   * @param {ICalldataDecoder} calldataDecoder - Decodes function calldata for filter evaluation.
   * @param {MessageSentEventProcessorConfig} config - Network-specific settings including listener parameters and feature flags.
   * @param {ILogger} logger - Used for logging messages.
   */
  constructor(
    private readonly messageRepository: IMessageRepository,
    private readonly logClient: IMessageSentEventLogClient,
    private readonly provider: IBlockProvider,
    private readonly calldataDecoder: ICalldataDecoder,
    protected readonly config: MessageSentEventProcessorConfig,
    private readonly logger: ILogger,
  ) {
    this.maxBlocksToFetchLogs = Math.max(config.maxBlocksToFetchLogs, 0);
  }

  /**
   * Calculates the starting block number for fetching events, ensuring it is within the valid range.
   */
  private calculateFromBlockNumber(fromBlockNumber: number, toBlockNumber: number): number {
    if (fromBlockNumber > toBlockNumber) {
      return toBlockNumber;
    }
    return Math.max(fromBlockNumber, 0);
  }

  /**
   * Fetches `MessageSent` events from the blockchain within a specified block range and stores them in the database.
   */
  public async process(
    fromBlock: number,
    fromBlockLogIndex: number,
  ): Promise<{ nextFromBlock: number; nextFromBlockLogIndex: number }> {
    const latestBlockNumber = Math.max((await this.provider.getBlockNumber()) - this.config.blockConfirmation, 0);
    const toBlock = Math.min(latestBlockNumber, fromBlock + this.maxBlocksToFetchLogs);

    fromBlock = this.calculateFromBlockNumber(fromBlock, toBlock);

    this.logger.info("Fetching events.", { fromBlock, toBlock });

    const events = await this.logClient.getMessageSentEvents({
      filters: {
        from: this.config.eventFilters?.fromAddressFilter,
        to: this.config.eventFilters?.toAddressFilter,
      },
      fromBlock: BigInt(fromBlock),
      toBlock: BigInt(toBlock),
      fromBlockLogIndex,
    });

    this.logger.info("Number of fetched MessageSent events.", { count: events.length });

    for (const event of events) {
      const shouldBeProcessed = this.shouldProcessMessage(
        event,
        event.messageHash,
        this.config.eventFilters?.calldataFilter,
      );
      const messageStatusToInsert = shouldBeProcessed ? MessageStatus.SENT : MessageStatus.EXCLUDED;

      const message = new Message({
        ...event,
        sentBlockNumber: event.blockNumber,
        direction: this.config.direction,
        status: messageStatusToInsert,
        claimNumberOfRetry: 0,
        claimCycleCount: 0,
      });

      await this.messageRepository.insertMessage(message);
    }
    this.logger.info("Messages hashes found.", { messageHashes: serialize(events.map((event) => event.messageHash)) });

    return { nextFromBlock: toBlock + 1, nextFromBlockLogIndex: 0 };
  }

  /**
   * Determines whether a message should be processed based on its calldata and the configuration.
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
      this.logger.debug("Message has been excluded because target address is not an EOA or calldata is not empty.", {
        messageHash,
      });
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

    const decodedCalldata = this.calldataDecoder.decode(filters.calldataFunctionInterface, event.calldata);
    const context = {
      calldata: {
        funcSignature: event.calldata.slice(0, 10),
        ...this.convertBigInts(decodedCalldata),
      },
    };

    const passesFilter = this.evaluateExpression(filters.criteriaExpression, context);

    if (!passesFilter) {
      this.logger.debug("Message has been excluded because it does not match the criteria.", {
        criteria: filters.criteriaExpression,
        messageHash: event.messageHash,
        transactionHash: event.transactionHash,
      });
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
