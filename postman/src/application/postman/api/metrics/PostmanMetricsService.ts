import { SingletonMetricsService } from "@consensys/linea-shared-utils";

import { LineaPostmanMetrics } from "../../../../core/metrics/LineaPostmanMetrics";

export class PostmanMetricsService extends SingletonMetricsService<LineaPostmanMetrics> {
  constructor(defaultLabels: Record<string, string> = { app: "postman" }) {
    super(defaultLabels);
  }
}
