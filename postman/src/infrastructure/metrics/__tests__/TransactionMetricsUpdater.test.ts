import { IMetricsService } from "@consensys/linea-shared-utils";

import { ITransactionMetricsUpdater, LineaPostmanMetrics } from "../../../core/metrics";
import { PostmanMetricsService } from "../PostmanMetricsService";
import { TransactionMetricsUpdater } from "../TransactionMetricsUpdater";

describe("TransactionMetricsUpdater", () => {
  let transactionMetricsUpdater: ITransactionMetricsUpdater;
  let metricsService: IMetricsService<LineaPostmanMetrics>;

  beforeEach(() => {
    metricsService = new PostmanMetricsService();
    transactionMetricsUpdater = new TransactionMetricsUpdater(metricsService);
  });

  it("should get correct values after add histogram value", async () => {
    transactionMetricsUpdater.addTransactionProcessingTime("L1_TO_L2", 2);
    transactionMetricsUpdater.addTransactionProcessingTime("L1_TO_L2", 3);

    const histogramValues = await metricsService.getHistogramMetricsValues(
      LineaPostmanMetrics.TransactionProcessingTime,
    );
    expect(histogramValues?.values.length).toBe(11);
  });

  it("should record infura confirmation time histogram value", async () => {
    transactionMetricsUpdater.addTransactionInfuraConfirmationTime("L1_TO_L2", 1.5);
    transactionMetricsUpdater.addTransactionInfuraConfirmationTime("L1_TO_L2", 4);

    const histogramValues = await metricsService.getHistogramMetricsValues(
      LineaPostmanMetrics.TransactionInfuraConfirmationTime,
    );
    expect(histogramValues?.values.length).toBe(11);
  });
});
