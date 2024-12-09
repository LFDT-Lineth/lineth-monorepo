/* eslint-disable @typescript-eslint/no-explicit-any */
export interface ILogger {
  readonly name: string;
  info(message: any, ...params: any[]): void;
  error(message: any, ...params: any[]): void;
  warn(message: any, ...params: any[]): void;
  debug(message: any, ...params: any[]): void;
  warnOrError(message: any, ...params: any[]): void;
}
