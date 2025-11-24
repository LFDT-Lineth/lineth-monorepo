import { wait } from "@consensys/linea-sdk";
import { ILogger } from "@consensys/linea-shared-utils";
import { IPoller } from "../../core/services/pollers/IPoller";
import { IDatabaseCleaner } from "../../core/persistence/IDatabaseCleaner";
import { DBCleanerConfig } from "../../application/postman/persistence/config/types";

export class DatabaseCleaningPoller implements IPoller {
  private isPolling = false;
  private enabled: boolean;
  private pollingInterval: number;
  private msBeforeNowToDelete: number;

  /**
   * Constructs a new instance of the `DatabaseCleaningPoller`.
   *
   * @param {IDatabaseCleaner} databaseCleaner - An instance of a class implementing the `IDatabaseCleaner` interface, responsible for executing the cleanup logic.
   * @param {ILogger} logger - An instance of a class implementing the `ILogger` interface, used for logging messages related to the polling and cleanup process.
   * @param {DBCleanerConfig} [config] - Optional configuration settings for the poller, including whether it is enabled, the polling interval, and the age threshold for data deletion.
   */
  constructor(
    private readonly databaseCleaner: IDatabaseCleaner,
    private readonly logger: ILogger,
    config: DBCleanerConfig,
  ) {
    this.enabled = config.enabled;
    this.pollingInterval = config.cleaningInterval;
    this.msBeforeNowToDelete = config.daysBeforeNowToDelete * 24 * 60 * 60 * 1000;
  }

  /**
   * Starts the polling process, triggering periodic database cleanup operations based on the configured interval and deletion threshold.
   */
  public async start() {
    if (!this.enabled) {
      this.logger.warn("%s is disabled", this.logger.name);
      return;
    }

    if (this.isPolling) {
      this.logger.warn("%s has already started.", this.logger.name);
      return;
    }
    this.logger.info("Starting %s...", this.logger.name);
    this.isPolling = true;

    while (this.isPolling) {
      await this.databaseCleaner.databaseCleanerRoutine(this.msBeforeNowToDelete);
      await wait(this.pollingInterval);
    }
  }

  /**
   * Stops the polling process, halting any further database cleanup operations.
   */
  public stop() {
    this.logger.info("Stopping %s...", this.logger.name);
    this.isPolling = false;
    this.logger.info("%s stopped.", this.logger.name);
  }
}
