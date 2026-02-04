import { IMetricsService } from "@consensys/linea-shared-utils";
import axios from "axios";
import { mock, MockProxy } from "jest-mock-extended";
import { Registry } from "prom-client";

import { createLoggerMock } from "../../__tests__/helpers/factories";
import { ExpressApiApplication } from "../ExpressApiApplication";

enum ExampleMetrics {
  ExampleMetrics = "ExampleMetrics",
}

// Test constants
const TEST_PORT = 0; // Use port 0 to let OS assign available port
const METRICS_ENDPOINT = "/metrics";
const METRICS_CONTENT_TYPE = "text/plain; version=0.0.4; charset=utf-8";
const MOCKED_METRICS_RESPONSE = "mocked metrics";
const METRICS_ERROR_MESSAGE = "Failed to collect metrics";
const HTTP_STATUS_INTERNAL_SERVER_ERROR = 500;

const createRegistry = (overrides?: Partial<Registry>): Registry =>
  ({
    contentType: METRICS_CONTENT_TYPE,
    metrics: async () => MOCKED_METRICS_RESPONSE,
    ...overrides,
  }) as unknown as Registry;

describe("ExpressApiApplication", () => {
  let app: ExpressApiApplication;
  let metricsService: MockProxy<IMetricsService<ExampleMetrics>>;
  let logger: ReturnType<typeof createLoggerMock>;
  let serverPort: number;

  beforeEach(async () => {
    // Arrange
    metricsService = mock<IMetricsService<ExampleMetrics>>();
    logger = createLoggerMock();
    metricsService.getRegistry.mockReturnValue(createRegistry());
    app = new ExpressApiApplication(TEST_PORT, metricsService, logger);

    // Start server to get actual port
    await app.start();
    serverPort = (app as any).server.address().port;
  });

  afterEach(async () => {
    // Cleanup
    app.onBeforeStart = undefined;
    app.onAfterStart = undefined;
    app.onBeforeStop = undefined;
    app.onAfterStop = undefined;
    await app.stop();
    jest.clearAllMocks();
  });

  it("should return metrics with correct content type when metrics endpoint is requested", async () => {
    // Act
    const response = await axios.get(`http://localhost:${serverPort}${METRICS_ENDPOINT}`);

    // Assert
    expect(response.status).toBe(200);
    expect(response.headers["content-type"]).toContain(METRICS_CONTENT_TYPE);
    expect(response.data).toBe(MOCKED_METRICS_RESPONSE);
    expect(metricsService.getRegistry).toHaveBeenCalled();
  });

  it("should return 500 status when metrics collection fails", async () => {
    // Arrange
    const metricsError = new Error("metrics failure");
    metricsService.getRegistry.mockReturnValue(
      createRegistry({
        metrics: async () => {
          throw metricsError;
        },
      }),
    );

    // Act
    try {
      await axios.get(`http://localhost:${serverPort}${METRICS_ENDPOINT}`);
      fail("Expected request to fail");
    } catch (error: any) {
      // Assert
      expect(error.response.status).toBe(HTTP_STATUS_INTERNAL_SERVER_ERROR);
      expect(error.response.data).toEqual({ error: METRICS_ERROR_MESSAGE });
      expect(logger.warn).toHaveBeenCalledWith(METRICS_ERROR_MESSAGE, { error: metricsError });
    }
  });

  it("should execute lifecycle hooks when starting server", async () => {
    // Arrange
    await app.stop();
    const onBeforeStart = jest.fn();
    const onAfterStart = jest.fn();
    app.onBeforeStart = onBeforeStart;
    app.onAfterStart = onAfterStart;

    // Act
    await app.start();

    // Assert
    expect(onBeforeStart).toHaveBeenCalledTimes(1);
    expect(onAfterStart).toHaveBeenCalledTimes(1);
    expect(logger.info).toHaveBeenCalledWith(`Listening on port ${TEST_PORT}`);
  });

  it("should execute lifecycle hooks when stopping server", async () => {
    // Arrange
    const onBeforeStop = jest.fn();
    const onAfterStop = jest.fn();
    app.onBeforeStop = onBeforeStop;
    app.onAfterStop = onAfterStop;

    // Act
    await app.stop();

    // Assert
    expect(onBeforeStop).toHaveBeenCalledTimes(1);
    expect(onAfterStop).toHaveBeenCalledTimes(1);
    expect(logger.info).toHaveBeenCalledWith(`Closing API server on port ${TEST_PORT}`);
  });

  it("should execute before-stop hook when server was never started", async () => {
    // Arrange
    await app.stop(); // Stop the server started in beforeEach
    logger.info.mockClear(); // Clear previous logger calls
    const onBeforeStop = jest.fn();
    const onAfterStop = jest.fn();
    app.onBeforeStop = onBeforeStop;
    app.onAfterStop = onAfterStop;

    // Act
    await app.stop();

    // Assert
    expect(onBeforeStop).toHaveBeenCalledTimes(1);
    expect(onAfterStop).not.toHaveBeenCalled();
    expect(logger.info).not.toHaveBeenCalled();
  });

  it("should propagate error when server close fails", async () => {
    // Arrange
    const closeError = new Error("close failure");
    const serverMock = (app as any).server;
    const originalClose = serverMock.close.bind(serverMock);

    serverMock.close = jest.fn((callback?: (err?: Error | null) => void) => {
      callback?.(closeError);
      return serverMock;
    });

    const onBeforeStop = jest.fn();
    const onAfterStop = jest.fn();
    app.onBeforeStop = onBeforeStop;
    app.onAfterStop = onAfterStop;

    // Act & Assert
    await expect(app.stop()).rejects.toThrow(closeError);
    expect(onBeforeStop).toHaveBeenCalledTimes(1);
    expect(onAfterStop).not.toHaveBeenCalled();

    // Cleanup: restore original close to allow afterEach cleanup
    serverMock.close = originalClose;
    app.onBeforeStop = undefined;
    app.onAfterStop = undefined;
  });
});
