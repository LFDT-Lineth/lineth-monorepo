import { Direction } from "@consensys/linea-sdk";

export interface IMessageClaimingProcessor {
  process(): Promise<void>;
}

export type MessageClaimingProcessorConfig = {
  maxNonceDiff: number;
  feeRecipientAddress?: string;
  profitMargin: number;
  maxNumberOfRetries: number;
  retryDelayInSeconds: number;
  maxClaimGasLimit: bigint;
  direction: Direction;
  originContractAddress: string;
  claimViaAddress?: string;
};
