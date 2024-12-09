import { Direction, wait } from "@consensys/linea-sdk";
import { ILogger } from "../../core/utils/logging/ILogger";
import { IPoller } from "../../core/services/pollers/IPoller";
import { IMessageAnchoringProcessor } from "../../core/services/processors/IMessageAnchoringProcessor";

type MessageAnchoringPollerConfig = {
  direction: Direction;
  pollingInterval: number;
};

export class MessageAnchoringPoller implements IPoller {
  private isPolling = false;

  /**
   * Constructs a new instance of the `MessageAnchoringPoller`.
   *
   * @param {IMessageAnchoringProcessor} anchoringProcessor - An instance of a class implementing the `IMessageAnchoringProcessor` interface, responsible for the message anchoring logic.
   * @param {MessageAnchoringPollerConfig} config - Configuration settings for the poller, including the direction of message flow and the polling interval.
   * @param {ILogger} logger - An instance of a class implementing the `ILogger` interface, used for logging messages related to the polling process.
   */
  constructor(
    private readonly anchoringProcessor: IMessageAnchoringProcessor,
    private config: MessageAnchoringPollerConfig,
    private readonly logger: ILogger,
  ) {}

  /**
   * Starts the polling process, triggering periodic execution of the message anchoring logic.
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
      await this.anchoringProcessor.process();
      await wait(this.config.pollingInterval);
    }
  }

  /**
   * Stops the polling process, halting any further execution of the message anchoring logic.
   * Logs information about the stopping process.
   */
  public stop() {
    this.logger.info("Stopping %s %s...", this.config.direction, this.logger.name);
    this.isPolling = false;
    this.logger.info("%s %s stopped.", this.config.direction, this.logger.name);
  }
}
