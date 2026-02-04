import { ILogger } from "../../logging/ILogger";
import { attempt, tryResult } from "../errors";

const buildLogger = () => {
  const warnMock = jest.fn();
  const logger: ILogger = {
    name: "test",
    info: jest.fn(),
    error: jest.fn(),
    warn: warnMock,
    debug: jest.fn(),
  };
  return { logger, warnMock };
};

describe("tryResult", () => {
  it("wrap synchronous success value in Result", async () => {
    // Arrange
    const EXPECTED_VALUE = 42;

    // Act
    const result = tryResult(() => EXPECTED_VALUE);

    // Assert
    await result.match(
      (value) => {
        expect(value).toBe(EXPECTED_VALUE);
      },
      () => {
        throw new Error("expected success branch");
      },
    );
  });

  it("capture thrown error from async function", async () => {
    // Arrange
    const expectedError = new Error("boom");

    // Act
    const result = tryResult(async () => {
      throw expectedError;
    });

    // Assert
    await result.match(
      () => {
        throw new Error("expected error branch");
      },
      (error) => {
        expect(error).toBe(expectedError);
      },
    );
  });

  it("convert non-Error throw to Error instance", async () => {
    // Arrange
    const NON_ERROR_VALUE = "not-an-error";

    // Act
    const result = tryResult(() => {
      throw NON_ERROR_VALUE;
    });

    // Assert
    await result.match(
      () => {
        throw new Error("expected error branch");
      },
      (error) => {
        expect(error).toBeInstanceOf(Error);
        expect(error.message).toBe(NON_ERROR_VALUE);
      },
    );
  });
});

describe("attempt", () => {
  it("return successful result without logging", async () => {
    // Arrange
    const { logger, warnMock } = buildLogger();
    const EXPECTED_VALUE = "ok";
    const CONTEXT_MESSAGE = "should not log";

    // Act
    const result = attempt(logger, () => EXPECTED_VALUE, CONTEXT_MESSAGE);

    // Assert
    await result.match(
      (value) => {
        expect(value).toBe(EXPECTED_VALUE);
      },
      () => {
        throw new Error("expected success branch");
      },
    );

    expect(warnMock).not.toHaveBeenCalled();
  });

  it("log error with context message when operation fails", async () => {
    // Arrange
    const { logger, warnMock } = buildLogger();
    const expectedError = new Error("failure");
    const CONTEXT_MESSAGE = "operation failed";
    const EXPECTED_CALL_COUNT = 1;

    // Act
    const result = attempt(
      logger,
      () => {
        throw expectedError;
      },
      CONTEXT_MESSAGE,
    );

    // Assert
    await result.match(
      () => {
        throw new Error("expected error branch");
      },
      (error) => {
        expect(error).toBe(expectedError);
      },
    );

    expect(warnMock).toHaveBeenCalledTimes(EXPECTED_CALL_COUNT);
    expect(warnMock).toHaveBeenCalledWith(CONTEXT_MESSAGE, { error: expectedError });
  });

  it("propagate error when operation fails", async () => {
    // Arrange
    const { logger } = buildLogger();
    const expectedError = new Error("failure");

    // Act
    const result = attempt(
      logger,
      () => {
        throw expectedError;
      },
      "operation failed",
    );

    // Assert
    await result.match(
      () => {
        throw new Error("expected error branch");
      },
      (error) => {
        expect(error).toBe(expectedError);
      },
    );
  });

  it("coerce non-Error throw to Error instance", async () => {
    // Arrange
    const { logger } = buildLogger();
    const NON_ERROR_VALUE = "coerce me";

    // Act
    const result = attempt(
      logger,
      () => {
        throw NON_ERROR_VALUE;
      },
      "non-error throw",
    );

    // Assert
    await result.match(
      () => {
        throw new Error("expected error branch");
      },
      (error) => {
        expect(error).toBeInstanceOf(Error);
        expect(error.message).toBe(NON_ERROR_VALUE);
      },
    );
  });

  it("log coerced Error instance when non-Error value is thrown", async () => {
    // Arrange
    const { logger, warnMock } = buildLogger();
    const NON_ERROR_VALUE = "coerce me";
    const CONTEXT_MESSAGE = "non-error throw";
    const EXPECTED_CALL_COUNT = 1;
    const ERROR_PROPERTY_INDEX = 0;
    const CALL_CONTEXT_ARG_INDEX = 1;

    // Act
    const result = attempt(
      logger,
      () => {
        throw NON_ERROR_VALUE;
      },
      CONTEXT_MESSAGE,
    );

    // Assert
    await result.match(
      () => {
        throw new Error("expected error branch");
      },
      () => {
        // Error validation happens in logging assertions below
      },
    );

    expect(warnMock).toHaveBeenCalledTimes(EXPECTED_CALL_COUNT);
    expect(warnMock.mock.calls[ERROR_PROPERTY_INDEX][CALL_CONTEXT_ARG_INDEX]).toHaveProperty("error");
    expect(warnMock.mock.calls[ERROR_PROPERTY_INDEX][CALL_CONTEXT_ARG_INDEX].error).toBeInstanceOf(Error);
    expect(warnMock.mock.calls[ERROR_PROPERTY_INDEX][CALL_CONTEXT_ARG_INDEX].error.message).toBe(NON_ERROR_VALUE);
  });
});
