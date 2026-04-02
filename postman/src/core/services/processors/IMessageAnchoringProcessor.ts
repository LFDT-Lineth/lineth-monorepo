import { Direction } from "../../enums";

import type { Address } from "../../types/primitives";

export interface IMessageAnchoringProcessor {
  process(): Promise<void>;
}

export type MessageAnchoringProcessorConfig = {
  direction: Direction;
  maxFetchMessagesFromDb: number;
  originContractAddress: Address;
};
