export interface IPoller {
  start(): Promise<void>;
  stop(): void;
}
