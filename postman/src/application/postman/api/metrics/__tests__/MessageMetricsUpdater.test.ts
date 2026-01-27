import { Direction } from "@consensys/linea-sdk";
import { mock, MockProxy } from "jest-mock-extended";
import { EntityManager, SelectQueryBuilder } from "typeorm";

import { MessageStatus } from "../../../../../core/enums";
import { IMessageMetricsUpdater } from "../../../../../core/metrics";
import { MessageMetricsUpdater } from "../MessageMetricsUpdater";
import { PostmanMetricsService } from "../PostmanMetricsService";

describe("MessageMetricsUpdater", () => {
  let messageMetricsUpdater: IMessageMetricsUpdater;
  let mockEntityManager: MockProxy<EntityManager>;

  beforeEach(() => {
    mockEntityManager = mock<EntityManager>();
    const metricService = new PostmanMetricsService();
    messageMetricsUpdater = new MessageMetricsUpdater(mockEntityManager, metricService);
  });

  const getMessagesCountQueryResp = [
    { status: MessageStatus.SENT, direction: Direction.L1_TO_L2, count: "5" },
    {
      status: MessageStatus.CLAIMED_SUCCESS,
      direction: Direction.L1_TO_L2,
      count: "10",
    },
  ];

  it("should get correct gauge values after initialization", async () => {
    jest.spyOn(mockEntityManager, "maximum").mockResolvedValue(10);
    jest.spyOn(mockEntityManager, "createQueryBuilder").mockReturnValue({
      select: jest.fn().mockReturnThis(),
      addSelect: jest.fn().mockReturnThis(),
      groupBy: jest.fn().mockReturnThis(),
      addGroupBy: jest.fn().mockReturnThis(),
      where: jest.fn().mockReturnThis(),
      andWhere: jest.fn().mockReturnThis(),
      getRawMany: jest.fn().mockResolvedValue(getMessagesCountQueryResp),
    } as unknown as SelectQueryBuilder<any>);

    await messageMetricsUpdater.initialize();

    // Check if the gauge was updated
    expect(
      await messageMetricsUpdater.getMessageCount({
        status: MessageStatus.SENT,
        direction: Direction.L1_TO_L2,
      }),
    ).toBe(5);

    expect(
      await messageMetricsUpdater.getMessageCount({
        status: MessageStatus.CLAIMED_SUCCESS,
        direction: Direction.L1_TO_L2,
      }),
    ).toBe(10);
  });

  it("should get correct values after incrementMessageCount", async () => {
    messageMetricsUpdater.incrementMessageCount(
      {
        status: MessageStatus.PENDING,
        direction: Direction.L1_TO_L2,
      },
      10,
    );
    const gaugeValue = await messageMetricsUpdater.getMessageCount({
      status: MessageStatus.PENDING,
      direction: Direction.L1_TO_L2,
    });
    expect(gaugeValue).toBe(10);
  });

  it("should get correct values after decrementMessageCount", async () => {
    messageMetricsUpdater.incrementMessageCount(
      {
        status: MessageStatus.PENDING,
        direction: Direction.L1_TO_L2,
      },
      10,
    );
    messageMetricsUpdater.decrementMessageCount(
      {
        status: MessageStatus.PENDING,
        direction: Direction.L1_TO_L2,
      },
      5,
    );
    const gaugeValue = await messageMetricsUpdater.getMessageCount({
      status: MessageStatus.PENDING,
      direction: Direction.L1_TO_L2,
    });
    expect(gaugeValue).toBe(5);
  });
});
