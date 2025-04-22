import {
  EventSubscriber,
  EntitySubscriberInterface,
  InsertEvent,
  UpdateEvent,
  RemoveEvent,
  TransactionCommitEvent,
} from "typeorm";
import { Direction } from "@consensys/linea-sdk";
import { MessageEntity } from "../entities/Message.entity";
import { IMetricsService, LineaPostmanMetrics } from "../../../../core/metrics/IMetricsService";
import { ILogger } from "../../../../core/utils/logging/ILogger";
import { MessageStatus } from "../../../../core/enums";

@EventSubscriber()
export class MessageStatusSubscriber implements EntitySubscriberInterface<MessageEntity> {
  constructor(
    private readonly metricsService: IMetricsService,
    private readonly logger: ILogger,
  ) {}

  listenTo() {
    return MessageEntity;
  }

  async afterInsert(event: InsertEvent<MessageEntity>): Promise<void> {
    const { status, direction } = event.entity;
    this.updateMessageMetricsOnInsert(status, direction);
  }

  async afterUpdate(event: UpdateEvent<MessageEntity>): Promise<void> {
    if (!event.entity || !event.databaseEntity) return;

    const prevStatus = event.databaseEntity.status;
    const newStatus = event.entity.status;
    const direction = event.databaseEntity.direction;

    if (prevStatus !== newStatus) {
      await this.swapStatus(
        LineaPostmanMetrics.Messages,
        { status: prevStatus, direction },
        { status: newStatus, direction },
      );
    }
  }

  async afterRemove(event: RemoveEvent<MessageEntity>): Promise<void> {
    if (event.entity) {
      await this.updateMessageMetricsOnRemove(event.databaseEntity.status, event.databaseEntity.direction);
    }
  }

  async afterTransactionCommit(event: TransactionCommitEvent): Promise<void> {
    const updatedEntity = event.queryRunner?.data?.updatedEntity;
    if (updatedEntity) {
      await this.swapStatus(
        LineaPostmanMetrics.Messages,
        { status: updatedEntity.previousStatus, direction: updatedEntity.direction },
        { status: updatedEntity.newStatus, direction: updatedEntity.direction },
      );
    }
  }

  private async updateMessageMetricsOnInsert(messageStatus: MessageStatus, messageDirection: Direction): Promise<void> {
    try {
      const prevGaugeValue = await this.metricsService.getGaugeValue(LineaPostmanMetrics.Messages, {
        status: messageStatus,
        direction: messageDirection,
      });

      if (prevGaugeValue === undefined) {
        return;
      }

      this.metricsService.incrementGauge(LineaPostmanMetrics.Messages, {
        status: messageStatus,
        direction: messageDirection,
      });
    } catch (error) {
      this.logger.error("Failed to update metrics:", error);
    }
  }

  private async updateMessageMetricsOnRemove(messageStatus: MessageStatus, messageDirection: Direction): Promise<void> {
    try {
      const prevGaugeValue = await this.metricsService.getGaugeValue(LineaPostmanMetrics.Messages, {
        status: messageStatus,
        direction: messageDirection,
      });

      if (prevGaugeValue && prevGaugeValue > 0) {
        this.metricsService.decrementGauge(LineaPostmanMetrics.Messages, {
          status: messageStatus,
          direction: messageDirection,
        });
      }
    } catch (error) {
      this.logger.error("Failed to update metrics:", error);
    }
  }

  private async swapStatus(
    name: LineaPostmanMetrics,
    previous: Record<string, string>,
    next: Record<string, string>,
  ): Promise<void> {
    try {
      const [prevVal, newVal] = await Promise.all([
        this.metricsService.getGaugeValue(name, previous),
        this.metricsService.getGaugeValue(name, next),
      ]);

      if (prevVal && prevVal > 0) {
        this.metricsService.decrementGauge(name, previous);
      }

      if (newVal !== undefined) {
        this.metricsService.incrementGauge(name, next);
      }
    } catch (error) {
      this.logger.error("Metrics swap failed:", error);
    }
  }
}
