import { Direction } from "@consensys/linea-sdk";

export interface IMessageSentEventProcessor {
  process(
    fromBlock: number,
    fromBlockLogIndex: number,
  ): Promise<{ nextFromBlock: number; nextFromBlockLogIndex: number }>;
}
export type MessageSentEventProcessorConfig = {
  direction: Direction;
  maxBlocksToFetchLogs: number;
  blockConfirmation: number;
  isEOAEnabled: boolean;
  isCalldataEnabled: boolean;
  eventFilters?: {
    fromAddressFilter?: string;
    toAddressFilter?: string;
    calldataFilter?: {
      criteriaExpression: string;
      calldataFunctionInterface: string;
    };
  };
};
