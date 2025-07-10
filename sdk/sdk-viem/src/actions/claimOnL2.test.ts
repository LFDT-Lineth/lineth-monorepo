import { claimOnL2 } from "./claimOnL2";
import { Client, Transport, Chain, Account, Address, BaseError, zeroAddress, encodeFunctionData, Hex } from "viem";
import { sendTransaction } from "viem/actions";
import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";
import { linea } from "viem/chains";
import { TEST_ADDRESS_1, TEST_ADDRESS_2, TEST_TRANSACTION_HASH } from "../../tests/constants";

jest.mock("viem/actions", () => ({
  sendTransaction: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("claimOnL2", () => {
  const mockClient = (chainId?: number, account?: Account): MockClient =>
    ({
      chain: chainId ? { id: chainId } : undefined,
      account,
    }) as unknown as MockClient;

  const mockAccount: Account = {
    address: "0x1111111111111111111111111111111111111111",
    type: "json-rpc",
  } as Account;

  const l2MessageServiceAddress = getContractsAddressesByChainId(linea.id).messageService;
  const chainId = linea.id;
  const from = TEST_ADDRESS_1;
  const to = TEST_ADDRESS_2 as Address;
  const fee = 1n;
  const value = 2n;
  const calldata = "0x" as Hex;
  const messageNonce = 3n;
  const feeRecipient = "0x5555555555555555555555555555555555555555" as Address;

  beforeEach(() => {
    (sendTransaction as jest.Mock<ReturnType<typeof sendTransaction>>).mockResolvedValue(TEST_TRANSACTION_HASH);
  });

  afterEach(() => {
    jest.clearAllMocks();
    (sendTransaction as jest.Mock).mockReset();
  });

  it("throws if no account is provided", async () => {
    const client = mockClient(chainId, undefined);
    await expect(claimOnL2(client, { from, to, fee, value, calldata, messageNonce })).rejects.toThrow(BaseError);
  });

  it("throws if no chain id is found", async () => {
    const client = mockClient(undefined, mockAccount);
    await expect(
      claimOnL2(client, { from, to, fee, value, calldata, messageNonce, account: mockAccount }),
    ).rejects.toThrow(BaseError);
  });

  it("sends claimMessage transaction with all parameters", async () => {
    const client = mockClient(chainId, mockAccount);
    const result = await claimOnL2(client, {
      from,
      to,
      fee,
      value,
      calldata,
      messageNonce,
      feeRecipient,
      account: mockAccount,
    });

    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: l2MessageServiceAddress,
        account: mockAccount,
        data: encodeFunctionData({
          abi: [
            {
              inputs: [
                { internalType: "address", name: "_from", type: "address" },
                { internalType: "address", name: "_to", type: "address" },
                { internalType: "uint256", name: "_fee", type: "uint256" },
                { internalType: "uint256", name: "_value", type: "uint256" },
                { internalType: "address payable", name: "_feeRecipient", type: "address" },
                { internalType: "bytes", name: "_calldata", type: "bytes" },
                { internalType: "uint256", name: "_nonce", type: "uint256" },
              ],
              name: "claimMessage",
              outputs: [],
              stateMutability: "nonpayable",
              type: "function",
            },
          ],
          functionName: "claimMessage",
          args: [from, to, fee, value, feeRecipient, calldata, messageNonce],
        }),
      }),
    );
    expect(result).toBe(TEST_TRANSACTION_HASH);
  });

  it("defaults feeRecipient to zeroAddress if not provided", async () => {
    const client = mockClient(chainId, mockAccount);
    await claimOnL2(client, {
      from,
      to,
      fee,
      value,
      calldata,
      messageNonce,
      account: mockAccount,
    });
    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        data: expect.stringContaining(zeroAddress.slice(2)),
      }),
    );
  });
});
