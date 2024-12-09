import { Direction } from "@consensys/linea-sdk";

export interface IMessageClaimingPersister {
  process(): Promise<void>;
}

export type MessageClaimingPersisterConfig = {
  direction: Direction;
  messageSubmissionTimeout: number;
  maxTxRetries: number;
};
