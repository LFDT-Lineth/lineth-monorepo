export interface IOperationModeSelector {
  start(): Promise<void>;
  stop(): void;
}
