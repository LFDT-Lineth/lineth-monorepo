import { OnChainMessageStatus } from "@lfdt-lineth/sdk-core";
import { Client, Transport, Chain, Account } from "viem";
import { readContract } from "viem/actions";
import { linea, mainnet } from "viem/chains";

import { getL2ToL1MessageStatus } from "./getL2ToL1MessageStatus";
import { getMessageSentEvents } from "./getMessageSentEvents";
import { TEST_MESSAGE_HASH } from "../../tests/constants";
import { generateMessageSentLog } from "../../tests/utils";

jest.mock("viem/actions", () => ({
  readContract: jest.fn(),
}));
jest.mock("./getMessageSentEvents", () => ({
  getMessageSentEvents: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("getL2ToL1MessageStatus", () => {
  const mockClient = (chainId: number): MockClient =>
    ({
      chain: { id: chainId },
    }) as unknown as MockClient;

  const mockL2Client = (chainId: number): MockClient =>
    ({
      chain: { id: chainId },
    }) as unknown as MockClient;

  // Routes the two parallel view calls: `currentL2BlockNumber` (anchoring) and `isMessageClaimed`.
  const mockReadContract = ({
    currentL2BlockNumber,
    isMessageClaimed,
  }: {
    currentL2BlockNumber: bigint;
    isMessageClaimed: boolean;
  }) =>
    (readContract as jest.Mock).mockImplementation((_client, params: { functionName: string }) =>
      Promise.resolve(params.functionName === "isMessageClaimed" ? isMessageClaimed : currentL2BlockNumber),
    );

  afterEach(() => {
    jest.clearAllMocks();
    (readContract as jest.Mock).mockReset();
    (getMessageSentEvents as jest.Mock).mockReset();
  });

  it("throws if client.chain is not set", async () => {
    const client = {} as MockClient;
    const l2Client = mockL2Client(linea.id);
    await expect(getL2ToL1MessageStatus(client, { l2Client, messageHash: TEST_MESSAGE_HASH })).rejects.toThrow(
      [
        "No chain was provided to the request.",
        "Please provide a chain with the `chain` argument on the Action, or by supplying a `chain` to WalletClient.",
      ].join("\n"),
    );
  });

  it("throws if l2Client.chain is not set", async () => {
    const client = mockClient(mainnet.id);
    const l2Client = {} as MockClient;
    await expect(getL2ToL1MessageStatus(client, { l2Client, messageHash: TEST_MESSAGE_HASH })).rejects.toThrow(
      "No chain was provided to the Client.",
    );
  });

  it("throws if messageSentEvent is not found", async () => {
    const client = mockClient(mainnet.id);
    const l2Client = mockL2Client(linea.id);
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([]);
    await expect(getL2ToL1MessageStatus(client, { l2Client, messageHash: TEST_MESSAGE_HASH })).rejects.toThrow(
      `Message with hash ${TEST_MESSAGE_HASH} not found.`,
    );
  });

  it("returns CLAIMED if isMessageClaimed is true", async () => {
    const client = mockClient(mainnet.id);
    const l2Client = mockL2Client(linea.id);
    const messageSentLog = generateMessageSentLog();

    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    mockReadContract({ currentL2BlockNumber: messageSentLog.blockNumber, isMessageClaimed: true });
    const result = await getL2ToL1MessageStatus(client, { l2Client, messageHash: TEST_MESSAGE_HASH });
    expect(result).toBe(OnChainMessageStatus.CLAIMED);
  });

  it("returns CLAIMABLE if isMessageClaimed is false but the L2 block is anchored", async () => {
    const client = mockClient(mainnet.id);
    const l2Client = mockL2Client(linea.id);
    const messageSentLog = generateMessageSentLog();

    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    // currentL2BlockNumber has reached (>=) the message's L2 block => anchored => claimable.
    mockReadContract({ currentL2BlockNumber: messageSentLog.blockNumber + 1n, isMessageClaimed: false });
    const result = await getL2ToL1MessageStatus(client, { l2Client, messageHash: TEST_MESSAGE_HASH });
    expect(result).toBe(OnChainMessageStatus.CLAIMABLE);
  });

  it("returns UNKNOWN if isMessageClaimed is false and the L2 block is not yet anchored", async () => {
    const client = mockClient(mainnet.id);
    const l2Client = mockL2Client(linea.id);
    const messageSentLog = generateMessageSentLog();

    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    // currentL2BlockNumber is still below the message's L2 block => not anchored => unknown.
    mockReadContract({ currentL2BlockNumber: messageSentLog.blockNumber - 1n, isMessageClaimed: false });
    const result = await getL2ToL1MessageStatus(client, { l2Client, messageHash: TEST_MESSAGE_HASH });
    expect(result).toBe(OnChainMessageStatus.UNKNOWN);
  });
});
