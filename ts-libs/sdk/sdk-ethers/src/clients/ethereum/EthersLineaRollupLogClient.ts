import { LineaRollup, LineaRollup__factory } from "../../contracts/typechain";
import { TypedContractEvent, TypedDeferredTopicFilter, TypedEventLog } from "../../contracts/typechain/common";
import {
  L2MessagingBlockAnchoredEvent,
  MessageClaimedEvent,
  MessageSentEvent,
} from "../../contracts/typechain/LineaRollup";
import {
  MessageSentEventFilters,
  ILineaRollupLogClient,
  L2MessagingBlockAnchoredFilters,
  MessageClaimedFilters,
} from "../../core/clients/ethereum";
import { L2MessagingBlockAnchored, MessageClaimed, MessageSent } from "../../core/types";
import { isUndefined } from "../../core/utils";
import { BrowserProvider, Provider } from "../providers";

export class EthersLineaRollupLogClient implements ILineaRollupLogClient {
  private lineaRollup: LineaRollup;

  /**
   * Initializes a new instance of the `EthersLineaRollupLogClient`.
   *
   * @param {Provider | BrowserProvider} provider - The JSON RPC provider for interacting with the Ethereum network.
   * @param {string} contractAddress - The address of the Linea Rollup contract.
   */
  constructor(provider: Provider | BrowserProvider, contractAddress: string) {
    this.lineaRollup = LineaRollup__factory.connect(contractAddress, provider);
  }

  /**
   * Fetches event logs from the Linea Rollup contract based on the provided filters and block range.
   *
   * This generic method queries the Ethereum blockchain for events emitted by the Linea Rollup contract that match the given criteria. It filters the events further based on the optional parameters for block range and log index, ensuring that only relevant events are returned.
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
    const events = await this.lineaRollup.queryFilter(eventFilter, fromBlock, toBlock);
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
   * @param {Object} params - The parameters for fetching the events.
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
    const messageSentEventFilter = this.lineaRollup.filters.MessageSent(
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
   * Retrieves `L2MessagingBlockAnchored` events that match the given filters.
   *
   * @param {Object} params - The parameters for fetching the events.
   * @param {L2MessagingBlockAnchoredFilters} [params.filters] - The L2MessagingBlockAnchored event filters to apply.
   * @param {number} [params.fromBlock=0] - The starting block number. Defaults to `0` if not specified.
   * @param {string|number} [params.toBlock='latest'] - The ending block number. Defaults to `latest` if not specified.
   * @param {number} [params.fromBlockLogIndex] - The log index to start from within the `fromBlock`.
   * @returns {Promise<L2MessagingBlockAnchored[]>} A promise that resolves to an array of `L2MessagingBlockAnchored` events.
   */
  public async getL2MessagingBlockAnchoredEvents(params: {
    filters?: L2MessagingBlockAnchoredFilters;
    fromBlock?: number;
    toBlock?: string | number;
    fromBlockLogIndex?: number;
  }): Promise<L2MessagingBlockAnchored[]> {
    const { filters, fromBlock, toBlock, fromBlockLogIndex } = params;
    const l2MessagingBlockAnchoredFilter = this.lineaRollup.filters.L2MessagingBlockAnchored(filters?.l2Block);

    return (
      await this.getEvents<L2MessagingBlockAnchoredEvent.Event>(
        l2MessagingBlockAnchoredFilter,
        fromBlock,
        toBlock,
        fromBlockLogIndex,
      )
    ).map((event) => ({
      l2Block: event.args.l2Block,
      blockNumber: event.blockNumber,
      logIndex: event.index,
      contractAddress: event.address,
      transactionHash: event.transactionHash,
    }));
  }

  /**
   * Retrieves `MessageClaimed` events that match the given filters.
   *
   * @param {Object} params - The parameters for fetching the events.
   * @param {MessageClaimedFilters} [params.filters] - The MessageClaimed event filters to apply.
   * @param {number} [params.fromBlock=0] - The starting block number. Defaults to `0` if not specified.
   * @param {string|number} [params.toBlock='latest'] - The ending block number. Defaults to `latest` if not specified.
   * @param {number} [params.fromBlockLogIndex] - The log index to start from within the `fromBlock`.
   * @returns {Promise<MessageClaimed[]>} A promise that resolves to an array of `MessageClaimed` events.
   */
  public async getMessageClaimedEvents(params: {
    filters?: MessageClaimedFilters;
    fromBlock?: number;
    toBlock?: string | number;
    fromBlockLogIndex?: number;
  }): Promise<MessageClaimed[]> {
    const { filters, fromBlock, toBlock, fromBlockLogIndex } = params;
    const messageClaimedFilter = this.lineaRollup.filters.MessageClaimed(filters?.messageHash);

    return (
      await this.getEvents<MessageClaimedEvent.Event>(messageClaimedFilter, fromBlock, toBlock, fromBlockLogIndex)
    ).map((event) => ({
      messageHash: event.args._messageHash,
      blockNumber: event.blockNumber,
      logIndex: event.index,
      contractAddress: event.address,
      transactionHash: event.transactionHash,
    }));
  }
}
