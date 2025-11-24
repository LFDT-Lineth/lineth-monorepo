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
  it("wraps synchronous success values", async () => {
    await tryResult(() => 42).match(
      (value) => {
        expect(value).toBe(42);
      },
      () => {
        throw new Error("expected success branch");
      },
    );
  });

  it("captures thrown errors from async functions", async () => {
    const expectedError = new Error("boom");

    await tryResult(async () => {
      throw expectedError;
    }).match(
      () => {
        throw new Error("expected error branch");
      },
      (error) => {
        expect(error).toBe(expectedError);
      },
    );
  });

  it("converts non-Error throws into Error instances", async () => {
    await tryResult(() => {
      throw "not-an-error";
    }).match(
      () => {
        throw new Error("expected error branch");
      },
      (error) => {
        expect(error).toBeInstanceOf(Error);
        expect(error.message).toBe("not-an-error");
      },
    );
  });
});

describe("attempt", () => {
  it("returns successful results without logging", async () => {
    const { logger, warnMock } = buildLogger();

    await attempt(logger, () => "ok", "should not log").match(
      (value) => {
        expect(value).toBe("ok");
      },
      () => {
        throw new Error("expected success branch");
      },
    );

    expect(warnMock).not.toHaveBeenCalled();
  });

  it("logs and propagates errors", async () => {
    const { logger, warnMock } = buildLogger();
    const expectedError = new Error("failure");

    await attempt(
      logger,
      () => {
        throw expectedError;
      },
      "operation failed",
    ).match(
      () => {
        throw new Error("expected error branch");
      },
      (error) => {
        expect(error).toBe(expectedError);
      },
    );

    expect(warnMock).toHaveBeenCalledTimes(1);
    expect(warnMock).toHaveBeenCalledWith("operation failed", { error: expectedError });
  });

  it("logs coerced Error instances when non-Error values are thrown", async () => {
    const { logger, warnMock } = buildLogger();

    await attempt(
      logger,
      () => {
        throw "coerce me";
      },
      "non-error throw",
    ).match(
      () => {
        throw new Error("expected error branch");
      },
      (error) => {
        expect(error).toBeInstanceOf(Error);
        expect(error.message).toBe("coerce me");
      },
    );

    expect(warnMock).toHaveBeenCalledTimes(1);
    expect(warnMock.mock.calls[0][1]).toHaveProperty("error");
    expect(warnMock.mock.calls[0][1].error).toBeInstanceOf(Error);
    expect(warnMock.mock.calls[0][1].error.message).toBe("coerce me");
  });
});
