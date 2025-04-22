import { EntityManager, SelectQueryBuilder } from "typeorm";
import { Direction } from "@consensys/linea-sdk";
import { MessageMetricsService } from "../MessageMetricsService";
import { mock, MockProxy } from "jest-mock-extended";
import { MessageStatus } from "../../../../../core/enums";
import { LineaPostmanMetrics } from "../../../../../core/metrics/IMetricsService";

describe("MessageMetricsService", () => {
  let messageMetricsService: MessageMetricsService;
  let mockEntityManager: MockProxy<EntityManager>;

  beforeEach(() => {
    mockEntityManager = mock<EntityManager>();
    messageMetricsService = new MessageMetricsService(mockEntityManager);
  });

  it("should update gauges based on message status", async () => {
    jest.spyOn(mockEntityManager, "maximum").mockResolvedValue(10);
    jest.spyOn(mockEntityManager, "createQueryBuilder").mockReturnValue({
      select: jest.fn().mockReturnThis(),
      addSelect: jest.fn().mockReturnThis(),
      groupBy: jest.fn().mockReturnThis(),
      addGroupBy: jest.fn().mockReturnThis(),
      getRawMany: jest.fn().mockResolvedValue([
        { status: MessageStatus.SENT, direction: Direction.L1_TO_L2, count: 5 },
        { status: MessageStatus.CLAIMED_SUCCESS, direction: Direction.L1_TO_L2, count: 10 },
      ]),
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as unknown as SelectQueryBuilder<any>);

    await messageMetricsService.initialize();

    // Check if the gauge was updated
    expect(
      await messageMetricsService.getGaugeValue(LineaPostmanMetrics.Messages, {
        status: MessageStatus.SENT,
        direction: Direction.L1_TO_L2,
      }),
    ).toBe(5);

    expect(
      await messageMetricsService.getGaugeValue(LineaPostmanMetrics.Messages, {
        status: MessageStatus.CLAIMED_SUCCESS,
        direction: Direction.L1_TO_L2,
      }),
    ).toBe(10);
  });

  it("should return the correct gauge value", async () => {
    messageMetricsService.incrementGauge(
      LineaPostmanMetrics.Messages,
      {
        status: "processed",
        direction: Direction.L1_TO_L2,
      },
      10,
    );
    const gaugeValue = await messageMetricsService.getGaugeValue(LineaPostmanMetrics.Messages, {
      status: "processed",
      direction: Direction.L1_TO_L2,
    });
    expect(gaugeValue).toBe(10);
  });
});
