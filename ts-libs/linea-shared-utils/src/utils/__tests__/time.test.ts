import { getCurrentUnixTimestampSeconds, msToSeconds, wait } from "../time";

describe("getCurrentUnixTimestampSeconds", () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("return the floored Unix timestamp derived from Date.now()", () => {
    // Arrange
    const MOCK_TIMESTAMP_MS = 1_700_000_123_456;
    const EXPECTED_TIMESTAMP_SECONDS = Math.floor(MOCK_TIMESTAMP_MS / 1000);
    jest.spyOn(Date, "now").mockReturnValue(MOCK_TIMESTAMP_MS);

    // Act
    const result = getCurrentUnixTimestampSeconds();

    // Assert
    expect(result).toBe(EXPECTED_TIMESTAMP_SECONDS);
  });
});

describe("msToSeconds", () => {
  it("floor fractional seconds", () => {
    // Arrange
    const MILLISECONDS_WITH_FRACTIONAL_SECONDS = 1999;
    const EXPECTED_SECONDS = 1;

    // Act
    const result = msToSeconds(MILLISECONDS_WITH_FRACTIONAL_SECONDS);

    // Assert
    expect(result).toBe(EXPECTED_SECONDS);
  });

  it("handle exact seconds correctly", () => {
    // Arrange
    const TWO_SECONDS_IN_MS = 2000;
    const EXPECTED_SECONDS = 2;

    // Act
    const result = msToSeconds(TWO_SECONDS_IN_MS);

    // Assert
    expect(result).toBe(EXPECTED_SECONDS);
  });

  it("handle zero milliseconds", () => {
    // Arrange
    const ZERO_MILLISECONDS = 0;
    const EXPECTED_SECONDS = 0;

    // Act
    const result = msToSeconds(ZERO_MILLISECONDS);

    // Assert
    expect(result).toBe(EXPECTED_SECONDS);
  });
});

describe("wait", () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("not resolve before the specified timeout", async () => {
    // Arrange
    const TIMEOUT_MS = 1000;
    const TIME_BEFORE_TIMEOUT = 999;
    const onResolve = jest.fn();

    // Act
    void wait(TIMEOUT_MS).then(onResolve);
    jest.advanceTimersByTime(TIME_BEFORE_TIMEOUT);
    await Promise.resolve();

    // Assert
    expect(onResolve).not.toHaveBeenCalled();
  });

  it("resolve after the specified timeout", async () => {
    // Arrange
    const TIMEOUT_MS = 1000;
    const onResolve = jest.fn();

    // Act
    const waitPromise = wait(TIMEOUT_MS).then(onResolve);
    jest.advanceTimersByTime(TIMEOUT_MS);
    await Promise.resolve();

    // Assert
    expect(onResolve).toHaveBeenCalledTimes(1);
    await expect(waitPromise).resolves.toBeUndefined();
  });
});
