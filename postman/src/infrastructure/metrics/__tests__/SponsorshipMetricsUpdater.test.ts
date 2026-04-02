import { Direction } from "../../../core/enums";
import { ISponsorshipMetricsUpdater } from "../../../core/metrics";
import { PostmanMetricsService } from "../PostmanMetricsService";
import { SponsorshipMetricsUpdater } from "../SponsorshipMetricsUpdater";

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

  it("should return 0 for getSponsoredMessagesTotal when no increment has happened", async () => {
    const total = await sponsorshipMetricsUpdater.getSponsoredMessagesTotal(Direction.L2_TO_L1);
    expect(total).toBe(0);
  });

  it("should return 0n for getSponsorshipFeePaid when no increment has happened", async () => {
    const fee = await sponsorshipMetricsUpdater.getSponsorshipFeePaid(Direction.L2_TO_L1);
    expect(fee).toBe(0n);
  });
});
