import { Direction, wait } from "@consensys/linea-sdk";
import { ILogger } from "@consensys/linea-shared-utils";
import { IPoller } from "../../core/services/pollers/IPoller";
import { L2ClaimMessageTransactionSizeProcessor } from "../processors/L2ClaimMessageTransactionSizeProcessor";

type L2ClaimMessageTransactionSizePollerConfig = {
  pollingInterval: number;
};

export class L2ClaimMessageTransactionSizePoller implements IPoller {
  private isPolling = false;

  /**
   * Creates an instance of `L2ClaimMessageTransactionSizePoller`.
   *
   * @param {L2ClaimMessageTransactionSizeProcessor} transactionSizeProcessor - The processor for handling L2 claim message transaction sizes.
   * @param {L2ClaimMessageTransactionSizePollerConfig} config - The configuration for the poller.
   * @param {ILogger} logger - The logger for logging information and warnings.
   */
  constructor(
    private readonly transactionSizeProcessor: L2ClaimMessageTransactionSizeProcessor,
    private config: L2ClaimMessageTransactionSizePollerConfig,
    private readonly logger: ILogger,
  ) {}

  /**
   * Starts the poller.
   * Logs a warning if the poller is already running.
   * Continuously processes transaction sizes and waits for the specified polling interval.
   *
   * @returns {Promise<void>} A promise that resolves when the poller is started.
   */
  public async start(): Promise<void> {
    if (this.isPolling) {
      this.logger.warn("%s has already started.", this.logger.name);
      return;
    }
    this.logger.info("Starting %s %s...", Direction.L1_TO_L2, this.logger.name);
    this.isPolling = true;

    while (this.isPolling) {
      await this.transactionSizeProcessor.process();
      await wait(this.config.pollingInterval);
    }
  }

  /**
   * Stops the poller.
   * Logs information about the stopping process.
   */
  public stop() {
    this.logger.info("Stopping %s %s...", Direction.L1_TO_L2, this.logger.name);
    this.isPolling = false;
    this.logger.info("%s %s stopped.", Direction.L1_TO_L2, this.logger.name);
  }
}
