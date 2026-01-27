import { Direction, wait } from "@consensys/linea-sdk";
import { ILogger } from "@consensys/linea-shared-utils";

import { IPoller } from "../../core/services/pollers/IPoller";
import { IMessageClaimingProcessor } from "../../core/services/processors/IMessageClaimingProcessor";

type MessageClaimingPollerConfig = {
  direction: Direction;
  pollingInterval: number;
};

export class MessageClaimingPoller implements IPoller {
  private isPolling = false;

  /**
   * Constructs a new instance of the `MessageClaimingPoller`.
   *
   * @param {IMessageClaimingProcessor} claimingProcessor - An instance of a class implementing the `IMessageClaimingProcessor` interface, responsible for the message claiming logic.
   * @param {MessageClaimingPollerConfig} config - Configuration settings for the poller, including the direction of message flow and the polling interval.
   * @param {ILogger} logger - An instance of a class implementing the `ILogger` interface, used for logging messages related to the polling process.
   */
  constructor(
    private readonly claimingProcessor: IMessageClaimingProcessor,
    private config: MessageClaimingPollerConfig,
    private readonly logger: ILogger,
  ) {}

  /**
   * Starts the polling process, triggering periodic execution of the message claiming logic.
   * Logs a warning if the poller is already running.
   *
   * @returns {Promise<void>} A promise that resolves when the poller is started.
   */
  public async start(): Promise<void> {
    if (this.isPolling) {
      this.logger.warn("%s has already started.", this.logger.name);
      return;
    }
    this.logger.info("Starting %s %s...", this.config.direction, this.logger.name);
    this.isPolling = true;

    while (this.isPolling) {
      await this.claimingProcessor.process();
      await wait(this.config.pollingInterval);
    }
  }

  /**
   * Stops the polling process, halting any further execution of the message claiming logic.
   * Logs information about the stopping process.
   */
  public stop() {
    this.logger.info("Stopping %s %s...", this.config.direction, this.logger.name);
    this.isPolling = false;
    this.logger.info("%s %s stopped.", this.config.direction, this.logger.name);
  }
}
