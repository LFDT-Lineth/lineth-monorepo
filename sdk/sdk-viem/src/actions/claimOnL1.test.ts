import { claimOnL1 } from "./claimOnL1";
import { Client, Transport, Chain, Account, Address, BaseError, zeroAddress, encodeFunctionData, Hex } from "viem";
import { sendTransaction } from "viem/actions";
import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";
import { mainnet } from "viem/chains";
import { TEST_ADDRESS_1, TEST_ADDRESS_2, TEST_TRANSACTION_HASH } from "../../tests/constants";

jest.mock("viem/actions", () => ({
  sendTransaction: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("claimOnL1", () => {
  const mockClient = (chainId?: number, account?: Account): MockClient =>
    ({
      chain: chainId ? { id: chainId } : undefined,
      account,
    }) as unknown as MockClient;

  const mockAccount: Account = {
    address: "0x1111111111111111111111111111111111111111",
    type: "json-rpc",
  } as Account;

  const l1MessageServiceAddress = getContractsAddressesByChainId(mainnet.id).messageService;
  const chainId = mainnet.id;
  const from = TEST_ADDRESS_1;
  const to = TEST_ADDRESS_2;
  const fee = 1n;
  const value = 2n;
  const calldata = "0x" as Hex;
  const messageNonce = 3n;
  const feeRecipient = "0x5555555555555555555555555555555555555555" as Address;
  const messageProof = {
    root: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" as `0x${string}`,
    proof: [
      "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" as `0x${string}`,
      "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc" as `0x${string}`,
    ],
    leafIndex: 42,
  };

  beforeEach(() => {
    (sendTransaction as jest.Mock).mockResolvedValue(TEST_TRANSACTION_HASH);
  });

  afterEach(() => {
    jest.clearAllMocks();
    (sendTransaction as jest.Mock).mockReset();
  });

  it("throws if no account is provided", async () => {
    const client = mockClient(chainId, undefined);
    await expect(claimOnL1(client, { from, to, fee, value, calldata, messageNonce, messageProof })).rejects.toThrow(
      BaseError,
    );
  });

  it("throws if no chain id is found", async () => {
    const client = mockClient(undefined, mockAccount);
    await expect(
      claimOnL1(client, { from, to, fee, value, calldata, messageNonce, messageProof, account: mockAccount }),
    ).rejects.toThrow(BaseError);
  });

  it("sends claimMessageWithProof transaction with all parameters", async () => {
    const client = mockClient(chainId, mockAccount);
    const result = await claimOnL1(client, {
      from,
      to,
      fee,
      value,
      calldata,
      messageNonce,
      feeRecipient,
      messageProof,
      account: mockAccount,
    });

    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: l1MessageServiceAddress,
        account: mockAccount,
        data: encodeFunctionData({
          abi: [
            {
              inputs: [
                {
                  components: [
                    { internalType: "bytes32[]", name: "proof", type: "bytes32[]" },
                    { internalType: "uint256", name: "messageNumber", type: "uint256" },
                    { internalType: "uint32", name: "leafIndex", type: "uint32" },
                    { internalType: "address", name: "from", type: "address" },
                    { internalType: "address", name: "to", type: "address" },
                    { internalType: "uint256", name: "fee", type: "uint256" },
                    { internalType: "uint256", name: "value", type: "uint256" },
                    { internalType: "address payable", name: "feeRecipient", type: "address" },
                    { internalType: "bytes32", name: "merkleRoot", type: "bytes32" },
                    { internalType: "bytes", name: "data", type: "bytes" },
                  ],
                  internalType: "struct IL1MessageService.ClaimMessageWithProofParams",
                  name: "_params",
                  type: "tuple",
                },
              ],
              name: "claimMessageWithProof",
              outputs: [],
              stateMutability: "nonpayable",
              type: "function",
            },
          ],
          functionName: "claimMessageWithProof",
          args: [
            {
              from,
              to,
              fee,
              value,
              feeRecipient,
              data: calldata,
              messageNumber: messageNonce,
              merkleRoot: messageProof.root,
              proof: messageProof.proof,
              leafIndex: messageProof.leafIndex,
            },
          ],
        }),
      }),
    );
    expect(result).toBe(TEST_TRANSACTION_HASH);
  });

  it("defaults feeRecipient to zeroAddress if not provided", async () => {
    const client = mockClient(chainId, mockAccount);
    await claimOnL1(client, {
      from,
      to,
      fee,
      value,
      calldata,
      messageNonce,
      messageProof,
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
