import { ILogger } from "@lfdt-lineth/shared-utils";

import { IPoller } from "../../core/services/pollers/IPoller";
import { wait } from "../../core/utils/shared";

export interface IProcessable {
  process(): Promise<void>;
}

type IntervalPollerConfig = {
  pollingInterval: number;
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
      this.logger.warn("Poller has already started.");
      return;
    }
    this.logger.info("Starting poller.", { pollingInterval: this.config.pollingInterval });
    this.isPolling = true;

    while (this.isPolling) {
      try {
        await this.processor.process();
      } catch (error) {
        this.logger.error("Unhandled error in polling loop — continuing.", { error });
      }
      await wait(this.config.pollingInterval);
    }
  }

  public stop() {
    this.logger.info("Stopping poller.");
    this.isPolling = false;
    this.logger.info("Poller stopped.");
  }
}
