export interface ITransactionMetricsUpdater {
  addTransactionProcessingTime(transactionProcessingTimeInSeconds: number): void;
}
