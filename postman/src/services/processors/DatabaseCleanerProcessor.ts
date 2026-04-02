import { ILogger } from "@consensys/linea-shared-utils";

import { IMessageRepository } from "../../core/persistence/IMessageRepository";
import {
  DatabaseCleanerProcessorConfig,
  IDatabaseCleanerProcessor,
} from "../../core/services/processors/IDatabaseCleanerProcessor";

export class DatabaseCleanerProcessor implements IDatabaseCleanerProcessor {
  private msBeforeNowToDelete: number;
  constructor(
    private readonly messageRepository: IMessageRepository,
    config: DatabaseCleanerProcessorConfig,
    private readonly logger: ILogger,
  ) {
    this.msBeforeNowToDelete = config.daysBeforeNowToDelete * 24 * 60 * 60 * 1000;
  }

  /**
   * Executes the database cleanup routine to delete messages older than a specified duration.
   */
  public async process(): Promise<void> {
    try {
      const affected = await this.messageRepository.deleteMessages(this.msBeforeNowToDelete);
      this.logger.info("Database cleanup result: deleted %s rows", affected);
    } catch (e) {
      this.logger.error("Database cleanup failed.", { error: e });
    }
  }
}
