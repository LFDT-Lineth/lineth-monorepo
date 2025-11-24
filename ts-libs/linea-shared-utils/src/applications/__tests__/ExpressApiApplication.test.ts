import { mock, MockProxy } from "jest-mock-extended";
import { ExpressApiApplication } from "../ExpressApiApplication";
import { Request, Response } from "express";
import { ILogger, IMetricsService } from "@consensys/linea-shared-utils";
import { Registry } from "prom-client";

enum ExampleMetrics {
  ExampleMetrics = "ExampleMetrics",
}

const createRegistry = (overrides?: Partial<Registry>): Registry =>
  ({
    contentType: "text/plain; version=0.0.4; charset=utf-8",
    metrics: async () => "mocked metrics",
    ...overrides,
  }) as unknown as Registry;

const captureMetricsHandler = (appInstance: ExpressApiApplication) => {
  const expressApp = appInstance["app"] as any;
  const originalGet = expressApp.get.bind(expressApp);
  let handler: ((req: Request, res: Response) => Promise<void>) | undefined;

  expressApp.get = (path: string, ...handlers: any[]) => {
    if (path === "/metrics" && handlers.length > 0) {
      handler = handlers[handlers.length - 1];
    }
    return originalGet(path, ...handlers);
  };

  (appInstance as any).setupMetricsRoute();
  expressApp.get = originalGet;

  if (!handler) {
    throw new Error("Metrics handler could not be captured");
  }
  return handler;
};

const createResponseMock = () => {
  const res: Partial<Response> & {
    set: jest.Mock;
    end: jest.Mock;
    status: jest.Mock;
    json: jest.Mock;
  } = {
    set: jest.fn(),
    end: jest.fn(),
    json: jest.fn(),
    status: jest.fn(),
  };
  res.status.mockImplementation(() => res as Response);
  return res;
};

describe("ExpressApiApplication", () => {
  let app: ExpressApiApplication;
  let metricsService: MockProxy<IMetricsService<ExampleMetrics>>;
  let logger: MockProxy<ILogger>;

  beforeEach(() => {
    metricsService = mock<IMetricsService<ExampleMetrics>>();
    logger = mock<ILogger>();
    metricsService.getRegistry.mockReturnValue(createRegistry());
    app = new ExpressApiApplication(0, metricsService, logger);
  });

  afterEach(async () => {
    app.onBeforeStop = undefined;
    app.onAfterStop = undefined;
    await app.stop();
    jest.clearAllMocks();
  });

  it("wires metrics service through the /metrics route", async () => {
    const handler = captureMetricsHandler(app);
    const res = createResponseMock();

    await handler({} as Request, res as Response);

    expect(res.set).toHaveBeenCalledWith("Content-Type", "text/plain; version=0.0.4; charset=utf-8");
    expect(res.end).toHaveBeenCalledWith("mocked metrics");
    expect(res.status).not.toHaveBeenCalled();
    expect(metricsService.getRegistry).toHaveBeenCalled();
  });

  it("returns 500 and logs when metrics collection fails", async () => {
    const metricsError = new Error("metrics failure");
    metricsService.getRegistry.mockReturnValue(
      createRegistry({
        metrics: async () => {
          throw metricsError;
        },
      }),
    );

    const handler = captureMetricsHandler(app);
    const res = createResponseMock();

    await handler({} as Request, res as Response);

    expect(res.status).toHaveBeenCalledWith(500);
    expect(res.json).toHaveBeenCalledWith({ error: "Failed to collect metrics" });
    expect(logger.warn).toHaveBeenCalledWith("Failed to collect metrics", { error: metricsError });
  });

  it("runs lifecycle hooks and logs when starting and stopping", async () => {
    type ServerMock = {
      on: jest.MockedFunction<(event: string, handler: () => void) => ServerMock>;
      close: jest.MockedFunction<(callback?: (err?: Error | null) => void) => ServerMock>;
    };
    const serverMock = {
      on: jest.fn(),
      close: jest.fn(),
    } as ServerMock;
    serverMock.on.mockImplementation((event, handler) => {
      if (event === "listening") handler();
      return serverMock;
    });
    serverMock.close.mockImplementation((callback) => {
      callback?.();
      return serverMock;
    });
    const listenSpy = jest.spyOn(app["app"], "listen").mockImplementation(() => serverMock as any);

    const onBeforeStart = jest.fn();
    const onAfterStart = jest.fn();
    const onBeforeStop = jest.fn();
    const onAfterStop = jest.fn();

    app.onBeforeStart = onBeforeStart;
    app.onAfterStart = onAfterStart;
    app.onBeforeStop = onBeforeStop;
    app.onAfterStop = onAfterStop;

    await app.start();
    expect(onBeforeStart).toHaveBeenCalledTimes(1);
    expect(onAfterStart).toHaveBeenCalledTimes(1);
    expect(logger.info).toHaveBeenCalledWith("Listening on port 0");

    await app.stop();
    expect(onBeforeStop).toHaveBeenCalledTimes(1);
    expect(onAfterStop).toHaveBeenCalledTimes(1);
    expect(logger.info).toHaveBeenCalledWith("Closing API server on port 0");
    expect(app["server"]).toBeUndefined();

    listenSpy.mockRestore();
  });

  it("handles stop gracefully when the server was never started", async () => {
    const onBeforeStop = jest.fn();
    const onAfterStop = jest.fn();
    app.onBeforeStop = onBeforeStop;
    app.onAfterStop = onAfterStop;

    await app.stop();

    expect(onBeforeStop).toHaveBeenCalledTimes(1);
    expect(onAfterStop).not.toHaveBeenCalled();
    expect(logger.info).not.toHaveBeenCalled();

    app.onBeforeStop = undefined;
    app.onAfterStop = undefined;
  });

  it("propagates errors from server.close and skips after-stop hook", async () => {
    type ServerMock = {
      on: jest.MockedFunction<(event: string, handler: () => void) => ServerMock>;
      close: jest.MockedFunction<(callback?: (err?: Error | null) => void) => ServerMock>;
    };
    const serverMock = {
      on: jest.fn(),
      close: jest.fn(),
    } as ServerMock;
    serverMock.on.mockImplementation((event, handler) => {
      if (event === "listening") handler();
      return serverMock;
    });
    const closeError = new Error("close failure");
    serverMock.close.mockImplementation((callback) => {
      callback?.(closeError);
      return serverMock;
    });
    const listenSpy = jest.spyOn(app["app"], "listen").mockImplementation(() => serverMock as any);

    const onBeforeStop = jest.fn();
    const onAfterStop = jest.fn();
    app.onBeforeStop = onBeforeStop;
    app.onAfterStop = onAfterStop;

    await app.start();

    await expect(app.stop()).rejects.toThrow(closeError);

    expect(onBeforeStop).toHaveBeenCalledTimes(1);
    expect(onAfterStop).not.toHaveBeenCalled();
    expect(logger.info).not.toHaveBeenCalledWith("Closing API server on port 0");
    expect(app["server"]).toBe(serverMock);

    // cleanup: allow stop to succeed so afterEach does not throw
    app.onBeforeStop = undefined;
    app.onAfterStop = undefined;
    serverMock.close.mockImplementation((callback) => {
      callback?.();
      return serverMock;
    });
    logger.info.mockClear();
    await app.stop().catch(() => {});
    listenSpy.mockRestore();
  });
});
