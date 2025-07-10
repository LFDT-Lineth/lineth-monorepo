import { getMessageByMessageHash } from "./getMessageByMessageHash";
import { Client, Transport, Chain, Account, Hex, BaseError } from "viem";
import { getContractEvents } from "viem/actions";
import { linea } from "viem/chains";
import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";
import { TEST_ADDRESS_1, TEST_ADDRESS_2, TEST_MESSAGE_HASH, TEST_TRANSACTION_HASH } from "../../tests/constants";
import { generateMessageSentLog } from "../../tests/utils";

jest.mock("viem/actions", () => ({
  getContractEvents: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("getMessageByMessageHash", () => {
  const mockClient = (chainId?: number): MockClient =>
    ({
      chain: chainId ? { id: chainId } : undefined,
    }) as unknown as MockClient;

  const messageHash: Hex = TEST_MESSAGE_HASH;

  afterEach(() => {
    jest.clearAllMocks();
    (getContractEvents as jest.Mock).mockReset();
  });

  it("throws if client.chain is not set", async () => {
    const client = mockClient();
    await expect(getMessageByMessageHash(client, { messageHash })).rejects.toThrow(BaseError);
  });

  it("throws if no event is found", async () => {
    const client = mockClient(linea.id);
    (getContractEvents as jest.Mock).mockResolvedValue([]);
    await expect(getMessageByMessageHash(client, { messageHash })).rejects.toThrow(
      `Message with hash ${messageHash} not found.`,
    );
  });

  it("returns message details if event is found", async () => {
    const client = mockClient(linea.id);
    const messageSentEvent = generateMessageSentLog({
      address: getContractsAddressesByChainId(linea.id).messageService,
    });

    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockResolvedValue([messageSentEvent]);

    const result = await getMessageByMessageHash(client, { messageHash });

    expect(result).toEqual({
      from: TEST_ADDRESS_1,
      to: TEST_ADDRESS_2,
      fee: 0n,
      value: 0n,
      nonce: 1n,
      calldata: "0x",
      messageHash,
      transactionHash: TEST_TRANSACTION_HASH,
      blockNumber: 100_000n,
    });

    expect(getContractEvents).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        address: getContractsAddressesByChainId(linea.id).messageService,
        eventName: "MessageSent",
        args: { _messageHash: messageHash },
      }),
    );
  });
});
