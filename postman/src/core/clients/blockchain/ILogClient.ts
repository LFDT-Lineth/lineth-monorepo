import { Address, Hash, MessageSent } from "../../types";

export type MessageSentEventFilters = {
  from?: Address;
  to?: Address;
  messageHash?: Hash;
};

export type GetMessageSentEventsParams = {
  filters?: MessageSentEventFilters;
  fromBlock?: bigint;
  toBlock?: string | bigint;
  fromBlockLogIndex?: number;
};

export interface IMessageSentEventLogClient {
  getMessageSentEvents(params: GetMessageSentEventsParams): Promise<MessageSent[]>;
}
