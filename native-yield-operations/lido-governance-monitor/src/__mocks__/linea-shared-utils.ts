import { jest } from "@jest/globals";

export interface ILogger {
  name: string;
  debug: (message: string, meta?: object) => void;
  error: (message: string, meta?: object) => void;
  info: (message: string, meta?: object) => void;
  warn: (message: string, meta?: object) => void;
  child: (context: Record<string, unknown>) => ILogger;
}

const createLoggerStub = (): ILogger => {
  const stub: ILogger = {
    name: "test-logger",
    debug: jest.fn(),
    error: jest.fn(),
    info: jest.fn(),
    warn: jest.fn(),
    child: jest.fn(() => stub),
  };
  return stub;
};

export const createLogger = jest.fn().mockImplementation(() => createLoggerStub());

export class WinstonLogger implements ILogger {
  public name: string;
  public debug = jest.fn();
  public error = jest.fn();
  public info = jest.fn();
  public warn = jest.fn();
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  public child = jest.fn((_context: Record<string, unknown>): ILogger => this);

  constructor(name: string, _options?: { level?: string }) {
    void _options; // Mock ignores options; required for call signature compatibility
    this.name = name;
  }
}

export const fetchWithTimeout = jest
  .fn<(url: string, options: RequestInit) => Promise<Response>>()
  .mockImplementation((url, options) => fetch(url, options));

// DiscourseFetcher defaults sleepFn to `wait` from this package; the real `wait` is not re-exported here,
// so without this mock default constructors that rely on `wait` get `undefined` and throw at runtime.
export const wait = jest.fn<() => Promise<void>>().mockResolvedValue(undefined);
