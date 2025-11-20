export interface ITransactionMetricsUpdater {
  addTransactionProcessingTime(direction: string, transactionProcessingTimeInSeconds: number): void;
  addTransactionInfuraConfirmationTime(direction: string, transactionBroadcastTimeInSeconds: number): void;
}
