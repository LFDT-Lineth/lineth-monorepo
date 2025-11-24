import type { Address, TransactionReceipt } from "viem";
import { DashboardABI } from "../../../core/abis/Dashboard.js";

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return {
    ...actual,
    parseEventLogs: jest.fn(),
  };
});

import { parseEventLogs } from "viem";

const mockedParseEventLogs = parseEventLogs as jest.MockedFunction<typeof parseEventLogs>;

let getNodeOperatorFeesPaidFromTxReceipt: typeof import("../getNodeOperatorFeesPaidFromTxReceipt.js").getNodeOperatorFeesPaidFromTxReceipt;

beforeAll(async () => {
  ({ getNodeOperatorFeesPaidFromTxReceipt } = await import("../getNodeOperatorFeesPaidFromTxReceipt.js"));
});

describe("getNodeOperatorFeesPaidFromTxReceipt", () => {
  const dashboardAddress = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" as Address;

  const buildReceipt = (logs: Array<{ address: string; data: string; topics: string[] }>): TransactionReceipt =>
    ({
      logs,
    }) as unknown as TransactionReceipt;

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("returns the fee from a FeeDisbursed event emitted by the dashboard", () => {
    const receipt = buildReceipt([
      {
        address: dashboardAddress,
        data: "0xfee",
        topics: ["0x01"],
      },
    ]);

    mockedParseEventLogs.mockReturnValueOnce([
      {
        eventName: "FeeDisbursed",
        args: { fee: 123n },
        address: dashboardAddress,
      } as any,
    ]);

    const fee = getNodeOperatorFeesPaidFromTxReceipt(receipt, dashboardAddress);

    expect(fee).toBe(123n);
    expect(mockedParseEventLogs).toHaveBeenCalledWith({
      abi: DashboardABI,
      eventName: "FeeDisbursed",
      logs: receipt.logs,
    });
  });

  it("returns zero when decoding fails for the dashboard log", () => {
    const receipt = buildReceipt([
      {
        address: dashboardAddress.toUpperCase(),
        data: "0xdead",
        topics: [],
      },
    ]);

    mockedParseEventLogs.mockReturnValueOnce([]);

    const fee = getNodeOperatorFeesPaidFromTxReceipt(receipt, dashboardAddress);

    expect(fee).toBe(0n);
    expect(mockedParseEventLogs).toHaveBeenCalledTimes(1);
  });

  it("ignores logs from other contracts or events and returns zero", () => {
    const receipt = buildReceipt([
      {
        address: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        data: "0x00",
        topics: [],
      },
      {
        address: dashboardAddress,
        data: "0x01",
        topics: [],
      },
    ]);

    mockedParseEventLogs.mockReturnValueOnce([
      {
        eventName: "FeeDisbursed",
        args: { fee: 456n },
        address: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      } as any,
    ]);

    const fee = getNodeOperatorFeesPaidFromTxReceipt(receipt, dashboardAddress);

    expect(fee).toBe(0n);
    expect(mockedParseEventLogs).toHaveBeenCalledTimes(1);
  });
});
