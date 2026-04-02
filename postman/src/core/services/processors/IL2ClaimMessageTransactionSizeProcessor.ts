import { Direction } from "../../enums";

import type { Address } from "../../types/primitives";

export interface IL2ClaimMessageTransactionSizeProcessor {
  process(): Promise<void>;
}

export type L2ClaimMessageTransactionSizeProcessorConfig = {
  direction: Direction;
  originContractAddress: Address;
};
