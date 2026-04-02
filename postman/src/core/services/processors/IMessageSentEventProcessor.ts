import { Direction } from "../../enums";

export interface IMessageSentEventProcessor {
  process(
    fromBlock: number,
    fromBlockLogIndex: number,
  ): Promise<{ nextFromBlock: number; nextFromBlockLogIndex: number }>;
}
import type { Address } from "../../types/primitives";

export type MessageSentEventProcessorConfig = {
  direction: Direction;
  maxBlocksToFetchLogs: number;
  blockConfirmation: number;
  isEOAEnabled: boolean;
  isCalldataEnabled: boolean;
  eventFilters?: {
    fromAddressFilter?: Address;
    toAddressFilter?: Address;
    calldataFilter?: {
      criteriaExpression: string;
      calldataFunctionInterface: string;
    };
  };
};
