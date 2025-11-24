import { Direction } from "@consensys/linea-sdk";
import { SponsorshipMetricsUpdater } from "../SponsorshipMetricsUpdater";
import { PostmanMetricsService } from "../PostmanMetricsService";
import { ISponsorshipMetricsUpdater } from "../../../../../core/metrics";

describe("SponsorshipMetricsUpdater", () => {
  let sponsorshipMetricsUpdater: ISponsorshipMetricsUpdater;

  beforeEach(() => {
    const metricService = new PostmanMetricsService();
    sponsorshipMetricsUpdater = new SponsorshipMetricsUpdater(metricService);
  });

  it("should get correct txFee after incrementing txFee", async () => {
    const txFeeA = 82821359154819519n;
    const txFeeB = 95357651471636n;
    await sponsorshipMetricsUpdater.incrementSponsorshipFeePaid(txFeeA, Direction.L1_TO_L2);
    await sponsorshipMetricsUpdater.incrementSponsorshipFeePaid(txFeeB, Direction.L1_TO_L2);
    const txFees = await sponsorshipMetricsUpdater.getSponsorshipFeePaid(Direction.L1_TO_L2);
    expect(txFees).toBe(txFeeA + txFeeB);
    expect(await sponsorshipMetricsUpdater.getSponsoredMessagesTotal(Direction.L1_TO_L2)).toBe(2);
  });
});
