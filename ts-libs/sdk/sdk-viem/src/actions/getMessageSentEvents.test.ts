import { Client, Transport, Chain, Account } from "viem";
import { getContractEvents } from "viem/actions";

import { getMessageSentEvents } from "./getMessageSentEvents";
import {
  TEST_ADDRESS_1,
  TEST_ADDRESS_2,
  TEST_CONTRACT_ADDRESS_1,
  TEST_MESSAGE_HASH,
  TEST_TRANSACTION_HASH,
} from "../../tests/constants";
import { generateMessageSentLog } from "../../tests/utils";

jest.mock("viem/actions", () => ({
  getContractEvents: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("getMessageSentEvents", () => {
  const mockClient = (chainId?: number): MockClient =>
    ({ chain: chainId ? { id: chainId } : undefined }) as unknown as MockClient;

  afterEach(() => {
    jest.clearAllMocks();
    (getContractEvents as jest.Mock).mockReset();
  });

  it("returns empty array if no events", async () => {
    const client = mockClient(1);
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockResolvedValue([]);
    const result = await getMessageSentEvents(client, { address: TEST_ADDRESS_1 });
    expect(result).toEqual([]);
  });

  it("returns parsed events if present", async () => {
    const client = mockClient(1);
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockResolvedValue([
      generateMessageSentLog(),
    ]);
    const result = await getMessageSentEvents(client, { address: TEST_ADDRESS_1 });
    expect(result).toStrictEqual([
      {
        messageSender: TEST_ADDRESS_1,
        destination: TEST_ADDRESS_2,
        fee: 0n,
        value: 0n,
        messageNonce: 1n,
        calldata: "0x",
        messageHash: TEST_MESSAGE_HASH,
        blockNumber: 100_000n,
        logIndex: 0,
        contractAddress: TEST_CONTRACT_ADDRESS_1,
        transactionHash: TEST_TRANSACTION_HASH,
      },
    ]);
  });

  it("filters out removed events", async () => {
    const client = mockClient(1);
    (getContractEvents as jest.Mock).mockResolvedValue([generateMessageSentLog({ removed: true })]);
    const result = await getMessageSentEvents(client, { address: TEST_ADDRESS_1 });
    expect(result).toEqual([]);
  });
});
