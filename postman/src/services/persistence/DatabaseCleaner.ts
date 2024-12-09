import { ContractTransactionResponse } from "ethers";
import { IMessageDBService } from "../../core/persistence/IMessageDBService";
import { IDatabaseCleaner } from "../../core/persistence/IDatabaseCleaner";
import { ILogger } from "../../core/utils/logging/ILogger";

export class DatabaseCleaner implements IDatabaseCleaner {
  /**
   * Constructs a new instance of the `DatabaseCleaner`.
   *
   * @param {IMessageDBService<ContractTransactionResponse>} databaseService - An instance of a MessageDBService that provides access to message storage and operations.
   * @param {ILogger} logger - An instance of a logger for logging information and errors during the cleanup process.
   */
  constructor(
    private readonly databaseService: IMessageDBService<ContractTransactionResponse>,
    private readonly logger: ILogger,
  ) {}

  /**
   * Executes the database cleanup routine to delete messages older than a specified duration.
   *
   * @param {number} msBeforeNowToDelete - The duration in milliseconds before the current time. Messages older than this duration will be deleted.
   */
  public async databaseCleanerRoutine(msBeforeNowToDelete: number) {
    try {
      const affected = await this.databaseService.deleteMessages(msBeforeNowToDelete);
      this.logger.info("Database cleanup result: deleted %s rows", affected);
    } catch (e) {
      this.logger.error(e);
    }
  }
}
