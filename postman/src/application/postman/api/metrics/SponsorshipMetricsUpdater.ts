import { IMetricsService, ISponsorshipMetricsUpdater, LineaPostmanMetrics } from "../../../../core/metrics";
import { Direction } from "@consensys/linea-sdk";

export class SponsorshipMetricsUpdater implements ISponsorshipMetricsUpdater {
  constructor(private readonly metricsService: IMetricsService) {
    this.metricsService.createCounter(
      LineaPostmanMetrics.SponsorshipFeesGwei,
      "Gwei component of tx fees paid for sponsored messages by direction",
      ["direction"],
    );
    this.metricsService.createCounter(
      LineaPostmanMetrics.SponsorshipFeesWei,
      "Wei component of tx fees paid for sponsored messages by direction",
      ["direction"],
    );
    this.metricsService.createCounter(
      LineaPostmanMetrics.SponsoredMessagesTotal,
      "Count of sponsored messages by direction",
      ["direction"],
    );
  }

  public async getSponsoredMessagesTotal(direction: Direction): Promise<number> {
    const total = await this.metricsService.getCounterValue(LineaPostmanMetrics.SponsoredMessagesTotal, { direction });
    if (total === undefined) return 0;
    return total;
  }

  public async getSponsorshipFeePaid(direction: Direction): Promise<bigint> {
    const wei = await this.metricsService.getCounterValue(LineaPostmanMetrics.SponsorshipFeesWei, { direction });
    const gwei = await this.metricsService.getCounterValue(LineaPostmanMetrics.SponsorshipFeesGwei, { direction });

    if (wei === undefined || gwei === undefined) return 0n;
    return BigInt(wei) + BigInt(gwei) * 1_000_000_000n;
  }

  public async incrementSponsorshipFeePaid(txFee: bigint, direction: Direction) {
    const { wei, gwei } = this.convertTxFeesToWeiAndGwei(txFee);
    await this.metricsService.incrementCounter(LineaPostmanMetrics.SponsoredMessagesTotal, { direction }, 1);
    await this.metricsService.incrementCounter(LineaPostmanMetrics.SponsorshipFeesWei, { direction }, wei);
    await this.metricsService.incrementCounter(LineaPostmanMetrics.SponsorshipFeesGwei, { direction }, gwei);
  }

  private convertTxFeesToWeiAndGwei(txFee: bigint): { gwei: number; wei: number } {
    // Last 9 digits
    const wei = Number(txFee % BigInt(1_000_000_000));
    const gwei = Number(txFee / BigInt(1_000_000_000));
    return { wei, gwei };
  }
}
