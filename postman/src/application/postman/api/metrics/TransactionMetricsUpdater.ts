import { IMetricsService, LineaPostmanMetrics, ITransactionMetricsUpdater } from "../../../../core/metrics";

export class TransactionMetricsUpdater implements ITransactionMetricsUpdater {
  constructor(private readonly metricsService: IMetricsService) {
    this.metricsService.createHistogram(
      LineaPostmanMetrics.TransactionProcessingTime,
      [0.1, 0.5, 1, 2, 3, 5, 7, 10],
      "Time taken to process a transaction",
    );
  }

  public addTransactionProcessingTime(transactionProcessingTimeInSeconds: number): void {
    return this.metricsService.addValueToHistogram(
      LineaPostmanMetrics.TransactionProcessingTime,
      transactionProcessingTimeInSeconds,
    );
  }
}
