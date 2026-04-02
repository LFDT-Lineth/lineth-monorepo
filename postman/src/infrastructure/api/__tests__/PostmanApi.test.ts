import { ExpressApiApplication } from "@consensys/linea-shared-utils";
import { describe, it, expect, beforeEach } from "@jest/globals";

import { mockLogger, mockMetricsService } from "../../../utils/testing/mocks";
import { createPostmanApi } from "../PostmanApi";

jest.mock("@consensys/linea-shared-utils", () => {
  const actual = jest.requireActual("@consensys/linea-shared-utils");
  return {
    ...actual,
    ExpressApiApplication: jest.fn(),
  };
});

describe("createPostmanApi", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("should create an ExpressApiApplication with the given port, metrics service, and logger", () => {
    const port = 8080;
    const metricsService = mockMetricsService();
    const logger = mockLogger();

    createPostmanApi(port, metricsService, logger);

    expect(ExpressApiApplication).toHaveBeenCalledWith(port, metricsService, logger);
  });

  it("should return the created application instance", () => {
    const mockApp = { start: jest.fn(), stop: jest.fn() };
    (ExpressApiApplication as jest.Mock).mockReturnValue(mockApp);

    const result = createPostmanApi(3000, mockMetricsService(), mockLogger());

    expect(result).toBe(mockApp);
  });
});
