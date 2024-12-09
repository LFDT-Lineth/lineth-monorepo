export interface IDatabaseCleaner {
  databaseCleanerRoutine(msBeforeNowToDelete: number): Promise<void>;
}
