export interface IDatabaseCleanerProcessor {
  process(): Promise<void>;
}

export type DatabaseCleanerProcessorConfig = {
  daysBeforeNowToDelete: number;
};
