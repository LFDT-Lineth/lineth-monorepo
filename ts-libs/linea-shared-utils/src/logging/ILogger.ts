/* eslint-disable @typescript-eslint/no-explicit-any */
export interface ILogger {
  readonly name: string;
  info(message: any, ...params: any[]): void;
  error(message: any, ...params: any[]): void;
  warn(message: any, ...params: any[]): void;
  debug(message: any, ...params: any[]): void;
  /**
   * Returns a derived logger that automatically merges `context` into every log entry.
   * Use this to attach static fields (direction, signerAddress, etc.) at wiring time
   * so every downstream call carries them without per-call repetition.
   */
  child(context: Record<string, unknown>): ILogger;
}
