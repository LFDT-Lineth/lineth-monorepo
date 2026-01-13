import { getMessagesByTransactionHash } from "./getMessagesByTransactionHash";
import { Client, Transport, Chain, Account, Hex, ChainNotFoundError } from "viem";
import { getTransactionReceipt } from "viem/actions";
import { linea } from "viem/chains";
import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";
import {
  TEST_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_MESSAGE_HASH,
  TEST_TRANSACTION_HASH,
} from "../../tests/constants";
import { generateTransactionReceipt } from "../../tests/utils";

jest.mock("viem/actions", () => ({
  getTransactionReceipt: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("getMessagesByTransactionHash", () => {
  const mockClient = (chainId?: number): MockClient =>
    ({
      chain: chainId ? { id: chainId } : undefined,
    }) as unknown as MockClient;

  const transactionHash: Hex = TEST_TRANSACTION_HASH;

  afterEach(() => {
    jest.clearAllMocks();
    (getTransactionReceipt as jest.Mock).mockReset();
  });

  it("throws if client.chain is not set", async () => {
    const client = mockClient();
    await expect(getMessagesByTransactionHash(client, { transactionHash })).rejects.toThrow(ChainNotFoundError);
  });

  it("returns empty array if no logs match", async () => {
    const client = mockClient(linea.id);
    (getTransactionReceipt as jest.Mock<ReturnType<typeof getTransactionReceipt>>).mockResolvedValue(
      generateTransactionReceipt({ logs: [] }),
    );
    const result = await getMessagesByTransactionHash(client, { transactionHash });
    expect(result).toEqual([]);
  });

  it("should use custom message service address when provided", async () => {
    const client = mockClient(linea.id);
    (getTransactionReceipt as jest.Mock<ReturnType<typeof getTransactionReceipt>>).mockResolvedValue(
      generateTransactionReceipt({ logs: [] }),
    );

    await getMessagesByTransactionHash(client, {
      transactionHash,
      messageServiceAddress: TEST_CONTRACT_ADDRESS_2,
    });
  });

  it("returns parsed messages if logs match", async () => {
    const client = mockClient(linea.id);
    const transactionReceipt = generateTransactionReceipt();
    (getTransactionReceipt as jest.Mock<ReturnType<typeof getTransactionReceipt>>).mockResolvedValue({
      ...transactionReceipt,
      logs: [
        {
          ...transactionReceipt.logs[0],
          address: getContractsAddressesByChainId(linea.id).messageService,
        },
      ],
    });

    const result = await getMessagesByTransactionHash(client, { transactionHash });
    expect(result).toStrictEqual([
      {
        from: TEST_ADDRESS_1,
        to: TEST_ADDRESS_1,
        fee: 1000000000000000n,
        value: 99000000000000000n,
        nonce: 983n,
        calldata: "0x",
        messageHash: TEST_MESSAGE_HASH,
        transactionHash,
        blockNumber: 100000n,
      },
    ]);
  });
});
