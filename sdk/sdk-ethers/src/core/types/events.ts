type EventLogBase = {
  blockNumber: number;
  logIndex: number;
  contractAddress: string;
  transactionHash: string;
};

export type MessageSent = {
  messageSender: string;
  destination: string;
  fee: bigint;
  value: bigint;
  messageNonce: bigint;
  calldata: string;
  messageHash: string;
} & EventLogBase;

export type L2MessagingBlockAnchored = {
  l2Block: bigint;
} & EventLogBase;

export type MessageClaimed = {
  messageHash: string;
} & EventLogBase;

export type ServiceVersionMigrated = {
  version: bigint;
} & EventLogBase;
