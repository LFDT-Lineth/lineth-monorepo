import { claimOnL1, ClaimOnL1Parameters } from "./claimOnL1";
import {
  Client,
  Transport,
  Chain,
  Account,
  Address,
  zeroAddress,
  encodeFunctionData,
  Hex,
  ChainNotFoundError,
  ClientChainNotConfiguredError,
} from "viem";
import { sendTransaction } from "viem/actions";
import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";
import { mainnet } from "viem/chains";
import {
  TEST_ADDRESS_1,
  TEST_ADDRESS_2,
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_MERKLE_ROOT,
  TEST_TRANSACTION_HASH,
} from "../../tests/constants";
import { AccountNotFoundError } from "../errors/account";
import { getMessageProof } from "./getMessageProof";
import { computeMessageHash } from "../utils/computeMessageHash";

jest.mock("viem/actions", () => ({
  sendTransaction: jest.fn(),
}));
jest.mock("./getMessageProof");

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
    root: TEST_MERKLE_ROOT as `0x${string}`,
    proof: [
      "0x0000000000000000000000000000000000000000000000000000000000000000",
      "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
      "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
      "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
      "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
    ] as Hex[],
    leafIndex: 0,
  };

  beforeEach(() => {
    (sendTransaction as jest.Mock).mockResolvedValue(TEST_TRANSACTION_HASH);
  });

  afterEach(() => {
    jest.clearAllMocks();
    (getMessageProof as jest.Mock).mockReset();
    (sendTransaction as jest.Mock).mockReset();
  });

  it("throws if no account is provided", async () => {
    const client = mockClient(chainId, undefined);
    await expect(claimOnL1(client, { from, to, fee, value, calldata, messageNonce, messageProof })).rejects.toThrow(
      AccountNotFoundError,
    );
  });

  it("throws if no chain id is found", async () => {
    const client = mockClient(undefined, mockAccount);
    await expect(
      claimOnL1(client, { from, to, fee, value, calldata, messageNonce, messageProof, account: mockAccount }),
    ).rejects.toThrow(ChainNotFoundError);
  });

  it("throws if no messageProof or l2Client is provided", async () => {
    const client = mockClient(chainId, mockAccount);
    await expect(
      claimOnL1(client, {
        from,
        to,
        fee,
        value,
        calldata,
        messageNonce,
        account: mockAccount,
      } as unknown as ClaimOnL1Parameters),
    ).rejects.toThrow("Either `messageProof` or `l2Client` must be provided to claim a message on L1.");
  });

  it("throws if no l2Client chain id is found", async () => {
    const client = mockClient(chainId, mockAccount);
    await expect(
      claimOnL1(client, {
        from,
        to,
        fee,
        value,
        calldata,
        messageNonce,
        account: mockAccount,
        l2Client: mockClient(undefined, mockAccount),
      }),
    ).rejects.toThrow(ClientChainNotConfiguredError);
  });

  it("sends claimMessageWithProof transaction with messageProof", async () => {
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

  it("sends claimMessageWithProof transaction without messageProof", async () => {
    const client = mockClient(chainId, mockAccount);
    const l2Client = mockClient(chainId, mockAccount);

    (getMessageProof as jest.Mock<ReturnType<typeof getMessageProof>>).mockResolvedValue(messageProof);

    const result = await claimOnL1(client, {
      from,
      to,
      fee,
      value,
      calldata,
      messageNonce,
      feeRecipient,
      l2Client,
      account: mockAccount,
    });

    expect(getMessageProof).toHaveBeenCalledWith(client, {
      l2Client,
      lineaRollupAddress: undefined,
      l2MessageServiceAddress: undefined,
      messageHash: computeMessageHash({
        from,
        to,
        fee,
        value,
        nonce: messageNonce,
        calldata,
      }),
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

  it("sends claimMessageWithProof transaction with custom contract addresses", async () => {
    const client = mockClient(chainId, mockAccount);
    const l2Client = mockClient(chainId, mockAccount);

    (getMessageProof as jest.Mock<ReturnType<typeof getMessageProof>>).mockResolvedValue(messageProof);

    const result = await claimOnL1(client, {
      from,
      to,
      fee,
      value,
      calldata,
      messageNonce,
      feeRecipient,
      l2Client,
      lineaRollupAddress: TEST_CONTRACT_ADDRESS_1,
      l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2,
      account: mockAccount,
    });

    expect(getMessageProof).toHaveBeenCalledWith(client, {
      l2Client,
      lineaRollupAddress: TEST_CONTRACT_ADDRESS_1,
      l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2,
      messageHash: computeMessageHash({
        from,
        to,
        fee,
        value,
        nonce: messageNonce,
        calldata,
      }),
    });

    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: TEST_CONTRACT_ADDRESS_1,
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
