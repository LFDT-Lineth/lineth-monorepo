import { MessageSent, ServiceVersionMigrated } from "../../types";

export type MessageSentEventFilters = {
  from?: string;
  to?: string;
  messageHash?: string;
};

export interface IL2MessageServiceLogClient {
  getMessageSentEvents(params: {
    filters?: MessageSentEventFilters;
    fromBlock?: number;
    toBlock?: string | number;
    fromBlockLogIndex?: number;
  }): Promise<MessageSent[]>;

  getMessageSentEventsByMessageHash(params: {
    messageHash: string;
    fromBlock?: number;
    toBlock?: string | number;
    fromBlockLogIndex?: number;
  }): Promise<MessageSent[]>;

  getMessageSentEventsByBlockRange(fromBlock: number, toBlock: number): Promise<MessageSent[]>;

  getServiceVersionMigratedEvents(param?: {
    fromBlock?: number;
    toBlock?: string | number;
    fromBlockLogIndex?: number;
  }): Promise<ServiceVersionMigrated[]>;
}
