import { Direction } from "../../enums";

import type { Address } from "../../types/primitives";

export interface IMessageClaimingProcessor {
  process(): Promise<void>;
}

export type MessageClaimingProcessorConfig = {
  feeRecipientAddress?: Address;
  profitMargin: number;
  maxNumberOfRetries: number;
  retryDelayInSeconds: number;
  maxClaimGasLimit: bigint;
  direction: Direction;
  originContractAddress: Address;
  claimViaAddress?: Address;
};
