import { getTransactionReceiptByMessageHash } from "./getTransactionReceiptByMessageHash";
import { Client, Transport, Chain, Account, BaseError } from "viem";
import { getContractEvents, getTransactionReceipt } from "viem/actions";
import { linea } from "viem/chains";
import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";
import { TEST_MESSAGE_HASH } from "../../tests/constants";
import { generateMessageSentLog, generateTransactionReceipt } from "../../tests/utils";

jest.mock("viem/actions", () => ({
  getContractEvents: jest.fn(),
  getTransactionReceipt: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("getTransactionReceiptByMessageHash", () => {
  const mockClient = (chainId?: number): MockClient =>
    ({
      chain: chainId ? { id: chainId } : undefined,
    }) as unknown as MockClient;

  afterEach(() => {
    jest.clearAllMocks();
    (getContractEvents as jest.Mock).mockReset();
    (getTransactionReceipt as jest.Mock).mockReset();
  });

  it("throws if client.chain is not set", async () => {
    const client = mockClient();
    await expect(getTransactionReceiptByMessageHash(client, { messageHash: TEST_MESSAGE_HASH })).rejects.toThrow(
      BaseError,
    );
  });

  it("throws if no event is found", async () => {
    const client = mockClient(linea.id);
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockResolvedValue([]);
    await expect(getTransactionReceiptByMessageHash(client, { messageHash: TEST_MESSAGE_HASH })).rejects.toThrow(
      `Message with hash ${TEST_MESSAGE_HASH} not found.`,
    );
  });

  it("returns receipt if event is found", async () => {
    const client = mockClient(linea.id);
    const event = generateMessageSentLog({
      address: getContractsAddressesByChainId(linea.id).messageService,
    });
    const receipt = generateTransactionReceipt();

    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockResolvedValue([event]);
    (getTransactionReceipt as jest.Mock<ReturnType<typeof getTransactionReceipt>>).mockResolvedValue(receipt);

    const result = await getTransactionReceiptByMessageHash(client, { messageHash: TEST_MESSAGE_HASH });
    expect(result).toBe(receipt);
    expect(getContractEvents).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        address: getContractsAddressesByChainId(linea.id).messageService,
        eventName: "MessageSent",
        args: { _messageHash: TEST_MESSAGE_HASH },
      }),
    );
    expect(getTransactionReceipt).toHaveBeenCalledWith(client, { hash: event.transactionHash });
  });
});
