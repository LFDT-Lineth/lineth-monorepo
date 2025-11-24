export interface IApplication {
  start(): Promise<void>;
  stop(): Promise<void>;
}
