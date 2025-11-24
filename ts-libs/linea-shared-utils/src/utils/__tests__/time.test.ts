import { getCurrentUnixTimestampSeconds, msToSeconds, wait } from "../time";

describe("getCurrentUnixTimestampSeconds", () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("returns the floored Unix timestamp derived from Date.now()", () => {
    const now = 1_700_000_123_456;
    jest.spyOn(Date, "now").mockReturnValue(now);

    const result = getCurrentUnixTimestampSeconds();
    expect(result).toBe(Math.floor(now / 1000));
  });
});

describe("msToSeconds", () => {
  it("floors fractional seconds", () => {
    expect(msToSeconds(1999)).toBe(1);
  });

  it("handles exact seconds", () => {
    expect(msToSeconds(2000)).toBe(2);
    expect(msToSeconds(0)).toBe(0);
  });
});

describe("wait", () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("resolves after the specified timeout", async () => {
    const onResolve = jest.fn();
    const promise = wait(1000).then(onResolve);
    jest.advanceTimersByTime(999);
    await Promise.resolve();
    expect(onResolve).not.toHaveBeenCalled();
    jest.advanceTimersByTime(1);
    await Promise.resolve();
    expect(onResolve).toHaveBeenCalledTimes(1);
    await expect(promise).resolves.toBeUndefined();
  });
});
