import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";
import {
  Client,
  Transport,
  Chain,
  Account,
  Address,
  Hex,
  zeroAddress,
  encodeFunctionData,
  ChainNotFoundError,
} from "viem";
import { readContract, sendTransaction } from "viem/actions";
import { linea } from "viem/chains";

import { withdraw } from "./withdraw";
import { TEST_TRANSACTION_HASH } from "../../tests/constants";
import { AccountNotFoundError } from "../errors/account";

jest.mock("viem/actions", () => ({
  readContract: jest.fn(),
  sendTransaction: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("withdraw", () => {
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
  const tokenBridgeAddress = getContractsAddressesByChainId(linea.id).tokenBridge;
  const chainId = linea.id;
  const to = "0x4444444444444444444444444444444444444444" as Address;
  const token = "0x5555555555555555555555555555555555555555" as Address;
  const amount = 123n;
  const data = "0x" as Hex;
  const minimumFeeInWei = 10n;

  beforeEach(() => {
    (readContract as jest.Mock<ReturnType<typeof readContract>>).mockResolvedValue(minimumFeeInWei);
    (sendTransaction as jest.Mock<ReturnType<typeof sendTransaction>>).mockResolvedValue(TEST_TRANSACTION_HASH);
  });

  afterEach(() => {
    jest.clearAllMocks();
    (readContract as jest.Mock).mockReset();
    (sendTransaction as jest.Mock).mockReset();
  });

  it("throws if no account is provided", async () => {
    const client = mockClient(chainId, undefined);
    await expect(withdraw(client, { token, to, amount })).rejects.toThrow(AccountNotFoundError);
  });

  it("throws if no chain id is found", async () => {
    const client = mockClient(undefined, mockAccount);
    await expect(withdraw(client, { token, to, amount, account: mockAccount })).rejects.toThrow(ChainNotFoundError);
  });

  it("sends ETH withdrawal transaction when token is zeroAddress", async () => {
    const client = mockClient(chainId, mockAccount);
    const result = await withdraw(client, {
      token: zeroAddress,
      to,
      amount,
      data,
      account: mockAccount,
    });
    expect(readContract).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        address: l2MessageServiceAddress,
        functionName: "minimumFeeInWei",
      }),
    );
    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: l2MessageServiceAddress,
        value: amount + minimumFeeInWei,
        account: mockAccount,
        data: encodeFunctionData({
          abi: [
            {
              inputs: [
                { internalType: "address", name: "_to", type: "address" },
                { internalType: "uint256", name: "_fee", type: "uint256" },
                { internalType: "bytes", name: "_calldata", type: "bytes" },
              ],
              name: "sendMessage",
              outputs: [],
              stateMutability: "payable",
              type: "function",
            },
          ],
          functionName: "sendMessage",
          args: [to, minimumFeeInWei, data],
        }),
      }),
    );
    expect(result).toBe(TEST_TRANSACTION_HASH);
  });

  it("sends ERC20 withdrawal transaction when token is not zeroAddress", async () => {
    const client = mockClient(chainId, mockAccount);
    const result = await withdraw(client, {
      token,
      to,
      amount,
      data,
      account: mockAccount,
    });
    expect(readContract).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        address: l2MessageServiceAddress,
        functionName: "minimumFeeInWei",
      }),
    );
    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: tokenBridgeAddress,
        value: minimumFeeInWei,
        account: mockAccount,
        data: encodeFunctionData({
          abi: [
            {
              inputs: [
                {
                  internalType: "address",
                  name: "_token",
                  type: "address",
                },
                {
                  internalType: "uint256",
                  name: "_amount",
                  type: "uint256",
                },
                {
                  internalType: "address",
                  name: "_recipient",
                  type: "address",
                },
              ],
              name: "bridgeToken",
              outputs: [],
              stateMutability: "payable",
              type: "function",
            },
          ],
          functionName: "bridgeToken",
          args: [token, amount, to],
        }),
      }),
    );
    expect(result).toBe(TEST_TRANSACTION_HASH);
  });

  it("defaults data to '0x' when not provided (ETH withdrawal)", async () => {
    const client = mockClient(chainId, mockAccount);
    const result = await withdraw(client, {
      token: zeroAddress,
      to,
      amount,
      account: mockAccount,
    });
    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: l2MessageServiceAddress,
        value: amount + minimumFeeInWei,
        account: mockAccount,
        data: expect.stringContaining("0x"),
      }),
    );
    expect(result).toBe(TEST_TRANSACTION_HASH);
  });
});
