import { mock, MockProxy } from "jest-mock-extended";
import type { IBlockchainClient, ILogger } from "@consensys/linea-shared-utils";
import type { PublicClient, TransactionReceipt, Address } from "viem";
import { DashboardABI } from "../../../core/abis/Dashboard.js";

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return {
    ...actual,
    getContract: jest.fn(),
    parseEventLogs: jest.fn(),
  };
});

import { getContract, parseEventLogs } from "viem";

const mockedGetContract = getContract as jest.MockedFunction<typeof getContract>;
const mockedParseEventLogs = parseEventLogs as jest.MockedFunction<typeof parseEventLogs>;

let DashboardContractClient: typeof import("../DashboardContractClient.js").DashboardContractClient;

beforeAll(async () => {
  ({ DashboardContractClient } = await import("../DashboardContractClient.js"));
});

describe("DashboardContractClient", () => {
  const contractAddress = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" as Address;

  let blockchainClient: MockProxy<IBlockchainClient<PublicClient, TransactionReceipt>>;
  let logger: MockProxy<ILogger>;
  let publicClient: PublicClient;
  const viemContractStub = {
    abi: DashboardABI,
    read: {
      obligations: jest.fn(),
    },
  } as any;

  beforeEach(() => {
    jest.clearAllMocks();
    blockchainClient = mock<IBlockchainClient<PublicClient, TransactionReceipt>>();
    logger = mock<ILogger>();
    publicClient = {} as PublicClient;
    blockchainClient.getBlockchainClient.mockReturnValue(publicClient);
    mockedGetContract.mockReturnValue(viemContractStub);
    // Clear static state before each test
    (DashboardContractClient as any).blockchainClient = undefined;
    (DashboardContractClient as any).logger = undefined;
    (DashboardContractClient as any).clientCache.clear();
  });

  const createClient = () => {
    // Initialize logger if not already initialized (needed for constructor)
    if (!(DashboardContractClient as any).logger) {
      DashboardContractClient.initialize(blockchainClient, logger);
    }
    return new DashboardContractClient(contractAddress);
  };

  const buildReceipt = (logs: Array<{ address: string; data: string; topics: string[] }>): TransactionReceipt =>
    ({
      logs,
    }) as unknown as TransactionReceipt;

  it("initializes the viem contract and exposes it through getters", () => {
    const client = createClient();

    expect(mockedGetContract).toHaveBeenCalledWith({
      abi: DashboardABI,
      address: contractAddress,
      client: publicClient,
    });
    expect(client.getAddress()).toBe(contractAddress);
    expect(client.getContract()).toBe(viemContractStub);
  });

  it("returns node operator fees when FeeDisbursed event is present", () => {
    const client = createClient();
    const receipt = buildReceipt([
      {
        address: "0x0000000000000000000000000000000000000000",
        data: "0x",
        topics: [],
      },
      {
        address: contractAddress,
        data: "0xdata",
        topics: ["0xtopic"],
      },
    ]);

    mockedParseEventLogs.mockReturnValueOnce([
      {
        eventName: "FeeDisbursed",
        args: { fee: 123n },
        address: contractAddress,
      } as any,
    ]);

    const fee = client.getNodeOperatorFeesPaidFromTxReceipt(receipt);

    expect(fee).toBe(123n);
    expect(mockedParseEventLogs).toHaveBeenCalledWith({
      abi: viemContractStub.abi,
      eventName: "FeeDisbursed",
      logs: receipt.logs,
    });
  });

  it("ignores logs that fail to decode and returns zero when no FeeDisbursed event", () => {
    const client = createClient();
    const receipt = buildReceipt([
      {
        address: contractAddress.toUpperCase(),
        data: "0xdead",
        topics: [],
      },
    ]);

    mockedParseEventLogs.mockReturnValueOnce([]);

    const fee = client.getNodeOperatorFeesPaidFromTxReceipt(receipt);

    expect(fee).toBe(0n);
    expect(mockedParseEventLogs).toHaveBeenCalledTimes(1);
  });

  it("returns zero when logs belong to other contracts or events", () => {
    const client = createClient();
    const receipt = buildReceipt([
      {
        address: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        data: "0xaaa",
        topics: [],
      },
      {
        address: contractAddress,
        data: "0xbb",
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

    const fee = client.getNodeOperatorFeesPaidFromTxReceipt(receipt);

    expect(fee).toBe(0n);
    expect(mockedParseEventLogs).toHaveBeenCalledTimes(1);
  });

  it("returns unpaid Lido protocol fees from obligations", async () => {
    const client = createClient();
    const sharesToBurn = 1000n;
    const feesToSettle = 500n;
    viemContractStub.read.obligations.mockResolvedValueOnce([sharesToBurn, feesToSettle]);

    const result = await client.peekUnpaidLidoProtocolFees();

    expect(result).toBe(feesToSettle);
    expect(viemContractStub.read.obligations).toHaveBeenCalledTimes(1);
    expect(viemContractStub.read.obligations).toHaveBeenCalledWith();
  });

  it("returns zero when there are no unpaid fees", async () => {
    const client = createClient();
    const sharesToBurn = 0n;
    const feesToSettle = 0n;
    viemContractStub.read.obligations.mockResolvedValueOnce([sharesToBurn, feesToSettle]);

    const result = await client.peekUnpaidLidoProtocolFees();

    expect(result).toBe(0n);
    expect(viemContractStub.read.obligations).toHaveBeenCalledTimes(1);
  });

  it("returns zero when feesToSettle is null", async () => {
    const client = createClient();
    const sharesToBurn = 1000n;
    const feesToSettle = null;
    viemContractStub.read.obligations.mockResolvedValueOnce([sharesToBurn, feesToSettle]);

    const result = await client.peekUnpaidLidoProtocolFees();

    expect(result).toBe(0n);
    expect(viemContractStub.read.obligations).toHaveBeenCalledTimes(1);
  });

  it("returns undefined and logs error when obligations call fails", async () => {
    DashboardContractClient.initialize(blockchainClient, logger);
    const client = createClient();
    const error = new Error("Contract call failed");
    viemContractStub.read.obligations.mockRejectedValueOnce(error);

    const result = await client.peekUnpaidLidoProtocolFees();

    expect(result).toBeUndefined();
    expect(logger.error).toHaveBeenCalledWith(`peekUnpaidLidoProtocolFees failed, error=${error}`);
    expect(viemContractStub.read.obligations).toHaveBeenCalledTimes(1);
  });

  describe("initialize", () => {
    it("sets the static blockchainClient and logger", () => {
      DashboardContractClient.initialize(blockchainClient, logger);

      expect((DashboardContractClient as any).blockchainClient).toBe(blockchainClient);
      expect((DashboardContractClient as any).logger).toBe(logger);
    });
  });

  describe("getOrCreate", () => {
    it("throws error when blockchainClient is not initialized", () => {
      expect(() => {
        DashboardContractClient.getOrCreate(contractAddress);
      }).toThrow(
        "DashboardContractClient: blockchainClient must be initialized via DashboardContractClient.initialize() before use",
      );
    });

    it("throws error when logger is not initialized", () => {
      (DashboardContractClient as any).blockchainClient = blockchainClient;
      (DashboardContractClient as any).logger = undefined;

      expect(() => {
        DashboardContractClient.getOrCreate(contractAddress);
      }).toThrow(
        "DashboardContractClient: logger must be initialized via DashboardContractClient.initialize() before use",
      );
    });

    it("creates and caches a new client when not cached", () => {
      DashboardContractClient.initialize(blockchainClient, logger);

      const client = DashboardContractClient.getOrCreate(contractAddress);

      expect(client).toBeInstanceOf(DashboardContractClient);
      expect(client.getAddress()).toBe(contractAddress);
      expect(mockedGetContract).toHaveBeenCalledWith({
        abi: DashboardABI,
        address: contractAddress,
        client: publicClient,
      });
    });

    it("returns cached client when already exists", () => {
      DashboardContractClient.initialize(blockchainClient, logger);

      const client1 = DashboardContractClient.getOrCreate(contractAddress);
      mockedGetContract.mockClear();
      const client2 = DashboardContractClient.getOrCreate(contractAddress);

      expect(client1).toBe(client2);
      expect(mockedGetContract).not.toHaveBeenCalled();
    });

    it("creates separate clients for different addresses", () => {
      DashboardContractClient.initialize(blockchainClient, logger);
      const otherAddress = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" as Address;

      const client1 = DashboardContractClient.getOrCreate(contractAddress);
      const client2 = DashboardContractClient.getOrCreate(otherAddress);

      expect(client1).not.toBe(client2);
      expect(client1.getAddress()).toBe(contractAddress);
      expect(client2.getAddress()).toBe(otherAddress);
    });
  });

  describe("constructor", () => {
    it("throws error when blockchainClient is not initialized", () => {
      expect(() => {
        new DashboardContractClient(contractAddress);
      }).toThrow(
        "DashboardContractClient: blockchainClient must be initialized via DashboardContractClient.initialize() before use",
      );
    });

    it("uses static blockchainClient when initialized", () => {
      DashboardContractClient.initialize(blockchainClient, logger);

      const client = new DashboardContractClient(contractAddress);

      expect(client).toBeInstanceOf(DashboardContractClient);
      expect(mockedGetContract).toHaveBeenCalledWith({
        abi: DashboardABI,
        address: contractAddress,
        client: publicClient,
      });
    });

    it("throws error when logger is not initialized", () => {
      (DashboardContractClient as any).blockchainClient = blockchainClient;
      (DashboardContractClient as any).logger = undefined;

      expect(() => {
        new DashboardContractClient(contractAddress);
      }).toThrow(
        "DashboardContractClient: logger must be initialized via DashboardContractClient.initialize() before use",
      );
    });
  });
});

