import { LineaPostmanMetrics } from "../../../../core/metrics/LineaPostmanMetrics";
import { SingletonMetricsService } from "@consensys/linea-shared-utils";

export class PostmanMetricsService extends SingletonMetricsService<LineaPostmanMetrics> {
  constructor(defaultLabels: Record<string, string> = { app: "postman" }) {
    super(defaultLabels);
  }
}
