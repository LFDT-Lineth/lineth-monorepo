import { SingletonMetricsService } from "../SingletonMetricsService";
import { ITransactionMetricsUpdater, LineaPostmanMetrics } from "../../../../../core/metrics";
import { TransactionMetricsUpdater } from "../TransactionMetricsUpdater";

describe("TransactionMetricsUpdater", () => {
  let transactionMetricsUpdater: ITransactionMetricsUpdater;
  let metricsService: SingletonMetricsService;

  beforeEach(() => {
    metricsService = new SingletonMetricsService();
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
});
