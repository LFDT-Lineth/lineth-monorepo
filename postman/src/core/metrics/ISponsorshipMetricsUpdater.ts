import { Direction } from "../enums";

export interface ISponsorshipMetricsUpdater {
  getSponsoredMessagesTotal(direction: Direction): Promise<number>;
  getSponsorshipFeePaid(direction: Direction): Promise<bigint>;
  incrementSponsorshipFeePaid(txFee: bigint, direction: Direction): Promise<void>;
}
