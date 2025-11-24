import { EntityManager } from "typeorm";
import { MessageEntity } from "../../persistence/entities/Message.entity";
import { IMessageMetricsUpdater, LineaPostmanMetrics, MessagesMetricsAttributes } from "../../../../core/metrics";
import { Direction } from "@consensys/linea-sdk";
import { MessageStatus } from "../../../../core/enums";
import { IMetricsService } from "@consensys/linea-shared-utils";

export class MessageMetricsUpdater implements IMessageMetricsUpdater {
  constructor(
    private readonly entityManager: EntityManager,
    private readonly metricsService: IMetricsService<LineaPostmanMetrics>,
  ) {
    this.metricsService.createGauge(
      LineaPostmanMetrics.Messages,
      "Current number of messages by status and direction",
      ["status", "direction"],
    );
  }

  public async initialize(): Promise<void> {
    this.initializeMessagesGauges();
  }

  private async initializeMessagesGauges(): Promise<void> {
    const totalNumberOfMessagesByAttributeGroups = await this.entityManager
      .createQueryBuilder(MessageEntity, "message")
      .select("message.status", "status")
      .addSelect("message.direction", "direction")
      .addSelect("COUNT(message.id)", "count") // Actually a string type
      .groupBy("message.status")
      .addGroupBy("message.direction")
      .getRawMany();

    // JSON.stringify(MessagesMetricsAttributes) => Count
    const resultMap = new Map<string, number>();

    totalNumberOfMessagesByAttributeGroups.forEach((r) => {
      const messageMetricAttributes: MessagesMetricsAttributes = {
        status: r.status,
        direction: r.direction,
      };
      const resultMapKey = JSON.stringify(messageMetricAttributes);
      resultMap.set(resultMapKey, parseInt(r.count));
    });

    // Note that we must initialize every attribute combination, or 'incrementGauge' and 'decrementGauge' will not work later on.
    for (const status of Object.values(MessageStatus)) {
      for (const direction of Object.values(Direction)) {
        const attributes: MessagesMetricsAttributes = {
          status,
          direction,
        };
        const attributesKey = JSON.stringify(attributes);
        this.metricsService.incrementGauge(
          LineaPostmanMetrics.Messages,
          {
            status: attributes.status,
            direction: attributes.direction,
          },
          resultMap.get(attributesKey) ?? 0,
        );
      }
    }
  }

  public async getMessageCount(messageAttributes: MessagesMetricsAttributes): Promise<number | undefined> {
    const { status, direction } = messageAttributes;
    return await this.metricsService.getGaugeValue(LineaPostmanMetrics.Messages, {
      status,
      direction,
    });
  }

  public async incrementMessageCount(messageAttributes: MessagesMetricsAttributes, value: number = 1) {
    const { status, direction } = messageAttributes;
    await this.metricsService.incrementGauge(
      LineaPostmanMetrics.Messages,
      {
        status,
        direction,
      },
      value,
    );
  }

  public async decrementMessageCount(messageAttributes: MessagesMetricsAttributes, value: number = 1) {
    const { status, direction } = messageAttributes;
    await this.metricsService.decrementGauge(
      LineaPostmanMetrics.Messages,
      {
        status,
        direction,
      },
      value,
    );
  }
}
