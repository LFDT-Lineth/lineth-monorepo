import { L2MessageService, L2MessageService__factory } from "../../contracts/typechain";
import { TypedContractEvent, TypedDeferredTopicFilter, TypedEventLog } from "../../contracts/typechain/common";
import { MessageSentEvent, ServiceVersionMigratedEvent } from "../../contracts/typechain/L2MessageService";
import { IL2MessageServiceLogClient, MessageSentEventFilters } from "../../core/clients/linea";
import { MessageSent, ServiceVersionMigrated } from "../../core/types";
import { isUndefined } from "../../core/utils";
import { LineaBrowserProvider, LineaProvider } from "../providers";

export class EthersL2MessageServiceLogClient implements IL2MessageServiceLogClient {
  private l2MessageService: L2MessageService;

  /**
   * Constructs a new instance of the `EthersL2MessageServiceLogClient`.
   *
   * @param {LineaProvider | LineaBrowserProvider} provider - The JSON RPC provider for interacting with the Ethereum network.
   * @param {string} contractAddress - The address of the L2 Message Service contract.
   */
  constructor(provider: LineaProvider | LineaBrowserProvider, contractAddress: string) {
    this.l2MessageService = L2MessageService__factory.connect(contractAddress, provider);
  }

  /**
   * Fetches event logs from the L2 Message Service contract based on the provided filters and block range.
   *
   * This generic method queries the Ethereum blockchain for events emitted by the L2 Message Service contract that match the given criteria. It filters the events further based on the optional parameters for block range and log index, ensuring that only relevant events are returned.
   *
   * @template TCEevent - A type parameter extending `TypedContractEvent`, representing the specific event type to fetch.
   * @param {TypedDeferredTopicFilter<TypedContractEvent>} eventFilter - The filter criteria used to select the events to be fetched. This includes the contract address, event signature, and any additional filter parameters.
   * @param {number} [fromBlock=0] - The block number from which to start fetching events. Defaults to `0` if not specified.
   * @param {string|number} [toBlock='latest'] - The block number until which to fetch events. Defaults to 'latest' if not specified.
   * @param {number} [fromBlockLogIndex] - The log index within the `fromBlock` from which to start fetching events. This allows for more granular control over the event fetch start point within a block.
   * @returns {Promise<TypedEventLog<TCEevent>[]>} A promise that resolves to an array of event logs of the specified type that match the filter criteria.
   */
  private async getEvents<TCEevent extends TypedContractEvent>(
    eventFilter: TypedDeferredTopicFilter<TypedContractEvent>,
    fromBlock?: number,
    toBlock?: string | number,
    fromBlockLogIndex?: number,
  ): Promise<TypedEventLog<TCEevent>[]> {
    const events = await this.l2MessageService.queryFilter(eventFilter, fromBlock, toBlock);
    return events
      .filter((event) => {
        if (isUndefined(fromBlockLogIndex) || isUndefined(fromBlock)) {
          return true;
        }
        if (event.blockNumber === fromBlock && event.index < fromBlockLogIndex) {
          return false;
        }
        return true;
      })
      .filter((e) => e.removed === false);
  }

  /**
   * Retrieves `MessageSent` events that match the given filters.
   *
   * @param {Object} params - The parameters for fetching events.
   * @param {MessageSentEventFilters} [params.filters] - The messageSent event filters to apply.
   * @param {number} [params.fromBlock=0] - The starting block number. Defaults to `0` if not specified.
   * @param {string|number} [params.toBlock='latest'] - The ending block number. Defaults to `latest` if not specified.
   * @param {number} [params.fromBlockLogIndex] - The log index to start from within the `fromBlock`.
   * @returns {Promise<MessageSent[]>} A promise that resolves to an array of `MessageSent` events.
   */
  public async getMessageSentEvents(params: {
    filters?: MessageSentEventFilters;
    fromBlock?: number;
    toBlock?: string | number;
    fromBlockLogIndex?: number;
  }): Promise<MessageSent[]> {
    const { filters, fromBlock, toBlock, fromBlockLogIndex } = params;
    const messageSentEventFilter = this.l2MessageService.filters.MessageSent(
      filters?.from,
      filters?.to,
      undefined,
      undefined,
      undefined,
      undefined,
      filters?.messageHash,
    );

    return (
      await this.getEvents<MessageSentEvent.Event>(messageSentEventFilter, fromBlock, toBlock, fromBlockLogIndex)
    ).map((event) => ({
      messageSender: event.args._from,
      destination: event.args._to,
      fee: event.args._fee,
      value: event.args._value,
      messageNonce: event.args._nonce,
      calldata: event.args._calldata,
      messageHash: event.args._messageHash,
      blockNumber: event.blockNumber,
      logIndex: event.index,
      contractAddress: event.address,
      transactionHash: event.transactionHash,
    }));
  }

  /**
   * Retrieves `MessageSent` events by message hash.
   *
   * @param {Object} params - The parameters for fetching events by message hash.
   * @param {string} params.messageHash - The hash of the message sent on L2.
   * @param {number} [params.fromBlock=0] - The starting block number. Defaults to `0` if not specified.
   * @param {string|number} [params.toBlock='latest'] - The ending block number. Defaults to `latest` if not specified.
   * @param {number} [params.fromBlockLogIndex] - The log index to start from within the `fromBlock`.
   * @returns {Promise<MessageSent[]>} A promise that resolves to an array of `MessageSent` events.
   */
  public async getMessageSentEventsByMessageHash(params: {
    messageHash: string;
    fromBlock?: number;
    toBlock?: string | number;
    fromBlockLogIndex?: number;
  }): Promise<MessageSent[]> {
    const { messageHash, fromBlock, toBlock, fromBlockLogIndex } = params;
    const messageSentEventFilter = this.l2MessageService.filters.MessageSent(
      undefined,
      undefined,
      undefined,
      undefined,
      undefined,
      undefined,
      messageHash,
    );

    return (
      await this.getEvents<MessageSentEvent.Event>(messageSentEventFilter, fromBlock, toBlock, fromBlockLogIndex)
    ).map((event) => ({
      messageSender: event.args._from,
      destination: event.args._to,
      fee: event.args._fee,
      value: event.args._value,
      messageNonce: event.args._nonce,
      calldata: event.args._calldata,
      messageHash: event.args._messageHash,
      blockNumber: event.blockNumber,
      logIndex: event.index,
      contractAddress: event.address,
      transactionHash: event.transactionHash,
    }));
  }

  /**
   * Retrieves `MessageSent` events within a specified block range.
   *
   * @param {number} fromBlock - The starting block number.
   * @param {number} toBlock - The ending block number.
   * @returns {Promise<MessageSent[]>} A promise that resolves to an array of `MessageSent` events.
   */
  public async getMessageSentEventsByBlockRange(fromBlock: number, toBlock: number): Promise<MessageSent[]> {
    const messageSentEventFilter = this.l2MessageService.filters.MessageSent();
    return (await this.getEvents<MessageSentEvent.Event>(messageSentEventFilter, fromBlock, toBlock)).map((event) => ({
      messageSender: event.args._from,
      destination: event.args._to,
      fee: event.args._fee,
      value: event.args._value,
      messageNonce: event.args._nonce,
      calldata: event.args._calldata,
      messageHash: event.args._messageHash,
      blockNumber: event.blockNumber,
      logIndex: event.index,
      contractAddress: event.address,
      transactionHash: event.transactionHash,
    }));
  }

  /**
   * Retrieves `ServiceVersionMigrated` events.
   *
   * @param {Object} [params] - The parameters for fetching the events.
   * @param {number} [params.fromBlock=0] - The starting block number. Defaults to `0` if not specified.
   * @param {string|number} [params.toBlock='latest'] - The ending block number. Defaults to `latest` if not specified.
   * @param {number} [params.fromBlockLogIndex] - The log index to start from within the `fromBlock`.
   * @returns {Promise<ServiceVersionMigrated[]>} A promise that resolves to an array of `ServiceVersionMigrated` events.
   */
  public async getServiceVersionMigratedEvents(params?: {
    fromBlock?: number;
    toBlock?: string | number;
    fromBlockLogIndex?: number;
  }): Promise<ServiceVersionMigrated[]> {
    const serviceVersionMigratedFilter = this.l2MessageService.filters.ServiceVersionMigrated(2);

    return (
      await this.getEvents<ServiceVersionMigratedEvent.Event>(
        serviceVersionMigratedFilter,
        params?.fromBlock,
        params?.toBlock,
        params?.fromBlockLogIndex,
      )
    ).map((event) => ({
      version: event.args.version,
      blockNumber: event.blockNumber,
      logIndex: event.index,
      contractAddress: event.address,
      transactionHash: event.transactionHash,
    }));
  }
}
