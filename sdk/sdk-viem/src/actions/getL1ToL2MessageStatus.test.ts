import { getL1ToL2MessageStatus } from "./getL1ToL2MessageStatus";
import { Client, Transport, Chain, Account, Hex } from "viem";
import { readContract } from "viem/actions";
import { linea } from "viem/chains";
import { OnChainMessageStatus } from "@consensys/linea-sdk-core";
import { TEST_MESSAGE_HASH } from "../../tests/constants";

jest.mock("viem/actions", () => ({
  readContract: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("getL1ToL2MessageStatus", () => {
  const mockClient = (chainId: number): MockClient =>
    ({
      chain: { id: chainId },
    }) as unknown as MockClient;

  afterEach(() => {
    jest.clearAllMocks();
    (readContract as jest.Mock).mockReset();
  });

  it("throws if client.chain is not set", async () => {
    const client = {} as MockClient;
    const messageHash: Hex = TEST_MESSAGE_HASH;
    await expect(getL1ToL2MessageStatus(client, { messageHash })).rejects.toThrow(
      [
        "No chain was provided to the request.",
        "Please provide a chain with the `chain` argument on the Action, or by supplying a `chain` to WalletClient.",
      ].join("\n"),
    );
  });

  it("returns UNKNOWN when readContract returns 0n", async () => {
    const client = mockClient(linea.id);
    const messageHash: Hex = TEST_MESSAGE_HASH;
    (readContract as jest.Mock<ReturnType<typeof readContract>>).mockResolvedValue(0n);
    const result = await getL1ToL2MessageStatus(client, { messageHash });
    expect(result).toBe(OnChainMessageStatus.UNKNOWN);
  });

  it("returns CLAIMABLE when readContract returns 1n", async () => {
    const client = mockClient(linea.id);
    const messageHash: Hex = TEST_MESSAGE_HASH;
    (readContract as jest.Mock<ReturnType<typeof readContract>>).mockResolvedValue(1n);
    const result = await getL1ToL2MessageStatus(client, { messageHash });
    expect(result).toBe(OnChainMessageStatus.CLAIMABLE);
  });

  it("returns CLAIMED when readContract returns 2n", async () => {
    const client = mockClient(linea.id);
    const messageHash: Hex = TEST_MESSAGE_HASH;
    (readContract as jest.Mock<ReturnType<typeof readContract>>).mockResolvedValue(2n);
    const result = await getL1ToL2MessageStatus(client, { messageHash });
    expect(result).toBe(OnChainMessageStatus.CLAIMED);
  });
});
