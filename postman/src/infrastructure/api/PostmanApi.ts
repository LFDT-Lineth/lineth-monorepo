import { ExpressApiApplication, IApplication, ILogger, IMetricsService } from "@lfdt-lineth/shared-utils";

import { LineaPostmanMetrics } from "../../core/metrics";

export function createPostmanApi(
  port: number,
  metricsService: IMetricsService<LineaPostmanMetrics>,
  logger: ILogger,
): IApplication {
  return new ExpressApiApplication(port, metricsService, logger);
}
