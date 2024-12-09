import { L2MessagingBlockAnchored, MessageClaimed, MessageSent } from "@consensys/linea-sdk";

export type MessageSentEventFilters = {
  from?: string;
  to?: string;
  messageHash?: string;
};

export type L2MessagingBlockAnchoredFilters = {
  l2Block: bigint;
};

export type MessageClaimedFilters = {
  messageHash: string;
};

export interface ILineaRollupLogClient {
  getMessageSentEvents(params: {
    filters?: MessageSentEventFilters;
    fromBlock?: number;
    toBlock?: string | number;
    fromBlockLogIndex?: number;
  }): Promise<MessageSent[]>;

  getL2MessagingBlockAnchoredEvents(params: {
    filters?: L2MessagingBlockAnchoredFilters;
    fromBlock?: number;
    toBlock?: string | number;
    fromBlockLogIndex?: number;
  }): Promise<L2MessagingBlockAnchored[]>;

  getMessageClaimedEvents(params: {
    filters?: MessageClaimedFilters;
    fromBlock?: number;
    toBlock?: string | number;
    fromBlockLogIndex?: number;
  }): Promise<MessageClaimed[]>;
}
