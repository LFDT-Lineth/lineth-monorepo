export interface IOperationLoop {
  start(): Promise<void>;
  stop(): void;
}
