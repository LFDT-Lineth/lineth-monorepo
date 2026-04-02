import { ILogger } from "@consensys/linea-shared-utils";

import { IPoller } from "../../core/services/pollers/IPoller";
import { wait } from "../../core/utils/shared";

export interface IProcessable {
  process(): Promise<void>;
}

type IntervalPollerConfig = {
  pollingInterval: number;
  direction?: string;
};

export class IntervalPoller implements IPoller {
  private isPolling = false;

  constructor(
    private readonly processor: IProcessable,
    private readonly config: IntervalPollerConfig,
    private readonly logger: ILogger,
  ) {}

  public async start(): Promise<void> {
    if (this.isPolling) {
      this.logger.warn("Poller has already started.", { name: this.logger.name });
      return;
    }
    this.logger.info("Starting poller.", {
      ...(this.config.direction ? { direction: this.config.direction } : {}),
      name: this.logger.name,
    });
    this.isPolling = true;

    while (this.isPolling) {
      try {
        await this.processor.process();
      } catch (error) {
        this.logger.error("Unhandled error in polling loop — continuing.", {
          ...(this.config.direction ? { direction: this.config.direction } : {}),
          name: this.logger.name,
          error,
        });
      }
      await wait(this.config.pollingInterval);
    }
  }

  public stop() {
    this.logger.info("Stopping poller.", {
      ...(this.config.direction ? { direction: this.config.direction } : {}),
      name: this.logger.name,
    });
    this.isPolling = false;
    this.logger.info("Poller stopped.", {
      ...(this.config.direction ? { direction: this.config.direction } : {}),
      name: this.logger.name,
    });
  }
}
