import { jest } from "@jest/globals";

export interface ILogger {
  name: string;
  debug: (message: string, meta?: object) => void;
  error: (message: string, meta?: object) => void;
  info: (message: string, meta?: object) => void;
  warn: (message: string, meta?: object) => void;
}

export const createLogger = jest.fn().mockReturnValue({
  name: "test-logger",
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
} as ILogger);

export class WinstonLogger implements ILogger {
  public name: string;
  public debug = jest.fn();
  public error = jest.fn();
  public info = jest.fn();
  public warn = jest.fn();

  constructor(name: string, _options?: { level?: string }) {
    void _options; // Mock ignores options; required for call signature compatibility
    this.name = name;
  }
}

export const fetchWithTimeout = jest.fn().mockImplementation((url: string, options: RequestInit) => {
  return fetch(url, options);
});

// DiscourseFetcher defaults sleepFn to `wait` from this package; the real `wait` is not re-exported here,
// so without this mock default constructors that rely on `wait` get `undefined` and throw at runtime.
export const wait = jest.fn().mockResolvedValue(undefined);
