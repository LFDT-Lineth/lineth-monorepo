import { deposit, computeMessageStorageSlot, createStateOverride } from "./deposit";
import {
  Client,
  Transport,
  Chain,
  Account,
  Address,
  zeroAddress,
  Hex,
  SendTransactionParameters,
  encodeFunctionData,
  toFunctionSelector,
  erc20Abi,
  ChainNotFoundError,
  ClientChainNotConfiguredError,
} from "viem";
import {
  readContract,
  sendTransaction,
  estimateFeesPerGas,
  estimateContractGas,
  multicall,
  waitForTransactionReceipt,
  getBlock,
} from "viem/actions";
import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";
import { linea, mainnet } from "viem/chains";
import { TEST_ADDRESS_1, TEST_ADDRESS_2, TEST_TRANSACTION_HASH } from "../../tests/constants";
import { generateBlock, generateTransactionReceipt } from "../../tests/utils";
import { computeMessageHash } from "../utils/computeMessageHash";
import { AccountNotFoundError } from "../errors/account";

jest.mock("viem/actions", () => ({
  readContract: jest.fn(),
  sendTransaction: jest.fn(),
  estimateFeesPerGas: jest.fn(),
  estimateContractGas: jest.fn(),
  multicall: jest.fn(),
  getBlock: jest.fn(),
  waitForTransactionReceipt: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("deposit", () => {
  const mockClient = (chainId?: number, account?: Account): MockClient =>
    ({
      chain: chainId ? { id: chainId } : undefined,
      account,
    }) as unknown as MockClient;

  const mockL2Client = (chainId?: number, account?: Account): MockClient =>
    ({
      chain: chainId ? { id: chainId } : undefined,
      account,
    }) as unknown as MockClient;

  const mockAccount: Account = {
    address: "0x1111111111111111111111111111111111111111",
    type: "json-rpc",
  } as Account;

  const l1ChainId = mainnet.id;
  const l2ChainId = linea.id;
  const l1MessageServiceAddress = getContractsAddressesByChainId(mainnet.id).messageService;
  const l1TokenBridgeAddress = getContractsAddressesByChainId(mainnet.id).tokenBridge;
  const to = TEST_ADDRESS_2;
  const token = "0x7777777777777777777777777777777777777777" as Address;
  const amount = 123n;
  const data = "0x" as Hex;
  const fee = 10n;
  const nextMessageNumber = 42n;
  const l2ClaimingTxGasLimit = 1000n;

  beforeEach(() => {
    (readContract as jest.Mock<ReturnType<typeof readContract>>).mockResolvedValue(nextMessageNumber);
    (sendTransaction as jest.Mock<ReturnType<typeof sendTransaction>>).mockResolvedValue(TEST_TRANSACTION_HASH);
    (estimateFeesPerGas as jest.Mock<ReturnType<typeof estimateFeesPerGas>>).mockResolvedValue({
      maxFeePerGas: 100_000n,
      maxPriorityFeePerGas: 99_000n,
    });
    (getBlock as jest.Mock<ReturnType<typeof getBlock>>).mockResolvedValue(generateBlock());
    (estimateContractGas as jest.Mock<ReturnType<typeof estimateContractGas>>).mockResolvedValue(l2ClaimingTxGasLimit);
  });

  afterEach(() => {
    jest.clearAllMocks();
    (readContract as jest.Mock).mockReset();
    (sendTransaction as jest.Mock).mockReset();
    (estimateFeesPerGas as jest.Mock).mockReset();
    (estimateContractGas as jest.Mock).mockReset();
    (multicall as jest.Mock).mockReset();
    (getBlock as jest.Mock).mockReset();
    (waitForTransactionReceipt as jest.Mock).mockReset();
  });

  it("throws if no account is provided", async () => {
    const client = mockClient(l1ChainId, undefined);
    await expect(deposit(client, { l2Client: mockL2Client(l2ChainId), token, to, amount })).rejects.toThrow(
      AccountNotFoundError,
    );
  });

  it("throws if no L1 chain id is found", async () => {
    const client = mockClient(undefined, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    await expect(deposit(client, { l2Client, token, to, amount, account: mockAccount })).rejects.toThrow(
      ChainNotFoundError,
    );
  });

  it("throws if no L2 chain id is found", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(undefined, mockAccount);
    await expect(deposit(client, { l2Client, token, to, amount, account: mockAccount })).rejects.toThrow(
      ClientChainNotConfiguredError,
    );
  });

  it("sends ETH deposit transaction when token is zeroAddress", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    const result = await deposit(client, {
      l2Client,
      token: zeroAddress,
      to,
      amount,
      data,
      account: mockAccount,
      fee,
    });
    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: l1MessageServiceAddress,
        value: amount + fee,
        account: mockAccount,
        data: expect.any(String),
      }),
    );
    expect(result).toBe(TEST_TRANSACTION_HASH);
  });

  it("sends ERC20 deposit transaction when token is not zeroAddress", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (multicall as jest.Mock)
      .mockResolvedValueOnce([
        {
          result: "TokenName",
          status: "success",
        },
        {
          result: "TKN",
          status: "success",
        },
        {
          result: 18,
          status: "success",
        },
        {
          result: zeroAddress,
          status: "success",
        },
      ])
      .mockResolvedValueOnce([200n, 200n]);

    const result = await deposit(client, {
      l2Client,
      token,
      to,
      amount,
      data,
      account: mockAccount,
      fee,
    });
    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: l1TokenBridgeAddress,
        value: fee,
        account: mockAccount,
        data: expect.any(String),
      }),
    );
    expect(result).toBe(TEST_TRANSACTION_HASH);
  });

  it("estimates fee if not provided (ETH)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    await deposit(client, {
      l2Client,
      token: zeroAddress,
      to,
      amount,
      data,
      account: mockAccount,
    });
    expect(estimateFeesPerGas).toHaveBeenCalledWith(l2Client, expect.any(Object));
    expect(readContract).toHaveBeenCalledWith(client, expect.objectContaining({ functionName: "nextMessageNumber" }));
    expect(estimateContractGas).toHaveBeenCalled();
  });

  it("estimates fee if not provided (ERC20)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (multicall as jest.Mock)
      .mockResolvedValueOnce([
        {
          result: "TokenName",
          status: "success",
        },
        {
          result: "TKN",
          status: "success",
        },
        {
          result: 18,
          status: "success",
        },
        {
          result: zeroAddress,
          status: "success",
        },
      ])
      .mockResolvedValueOnce([200n, 200n]);
    await deposit(client, {
      l2Client,
      token,
      to,
      amount,
      data,
      account: mockAccount,
    });
    expect(estimateFeesPerGas).toHaveBeenCalledWith(l2Client, expect.any(Object));
    expect(readContract).toHaveBeenCalledWith(client, expect.objectContaining({ functionName: "nextMessageNumber" }));
    expect(estimateContractGas).toHaveBeenCalled();
    expect(multicall).toHaveBeenCalledTimes(2);
  });

  it("calls approve if ERC20 allowance is insufficient", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (multicall as jest.Mock)
      .mockResolvedValueOnce([
        {
          result: "TokenName",
          status: "success",
        },
        {
          result: "TKN",
          status: "success",
        },
        {
          result: 18,
          status: "success",
        },
        {
          result: "0x1234567890123456789012345678901234567890", // bridgedToken
          status: "success",
        },
      ])
      .mockResolvedValueOnce([200n, 100n]); // allowance < amount
    (sendTransaction as jest.Mock)
      .mockResolvedValueOnce("APPROVE_TX_HASH")
      .mockResolvedValueOnce(TEST_TRANSACTION_HASH);
    (waitForTransactionReceipt as jest.Mock).mockResolvedValueOnce(generateTransactionReceipt());
    const result = await deposit(client, {
      l2Client,
      token,
      to,
      amount: 123n,
      data,
      account: mockAccount,
    });
    // Should call approve first, then deposit
    const sendTransactionMock = sendTransaction as jest.Mock;
    expect(sendTransactionMock).toHaveBeenCalledTimes(2);
    expect(sendTransactionMock.mock.calls[0][1]).toEqual(
      expect.objectContaining({
        to: token,
        data: encodeFunctionData({
          abi: erc20Abi,
          functionName: "approve",
          args: [l1TokenBridgeAddress, amount],
        }),
      }),
    );
    expect(sendTransactionMock.mock.calls[1][1]).toEqual(
      expect.objectContaining({
        to: l1TokenBridgeAddress,
        data: expect.any(String),
      }),
    );
    expect(result).toBe(TEST_TRANSACTION_HASH);
  });

  it("does not call approve if ERC20 allowance is sufficient", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (multicall as jest.Mock)
      .mockResolvedValueOnce([
        {
          result: "TokenName",
          status: "success",
        },
        {
          result: "TKN",
          status: "success",
        },
        {
          result: 18,
          status: "success",
        },
        {
          result: "0x1234567890123456789012345678901234567890", // bridgedToken
          status: "success",
        },
      ])
      .mockResolvedValueOnce([200n, 200n]); // allowance >= amount
    await deposit(client, {
      l2Client,
      token,
      to,
      amount: 123n,
      data,
      account: mockAccount,
    });
    // Only one sendTransaction call (the deposit)
    expect(sendTransaction).toHaveBeenCalledTimes(1);
    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: l1TokenBridgeAddress,
        data: expect.any(String),
      }),
    );
    // Check that approve was not called
    const approveCall = (sendTransaction as jest.Mock).mock.calls.find(
      ([, args]: [typeof client, SendTransactionParameters]) =>
        args.to === token &&
        args.data &&
        args.data.includes(
          toFunctionSelector({
            type: "function",
            name: "approve",
            stateMutability: "nonpayable",
            inputs: [
              {
                name: "spender",
                type: "address",
              },
              {
                name: "amount",
                type: "uint256",
              },
            ],
            outputs: [
              {
                type: "bool",
              },
            ],
          }),
        ),
    );
    expect(approveCall).toBeUndefined();
  });

  it("throws if ERC20 token balance is insufficient", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (multicall as jest.Mock)
      .mockResolvedValueOnce([
        {
          result: "TokenName",
          status: "success",
        },
        {
          result: "TKN",
          status: "success",
        },
        {
          result: 18,
          status: "success",
        },
        {
          result: "0x1234567890123456789012345678901234567890", // bridgedToken
          status: "success",
        },
      ])
      .mockResolvedValueOnce([100n, 200n]); // balance < amount
    await expect(
      deposit(client, {
        l2Client,
        token,
        to,
        amount: 123n,
        data,
        account: mockAccount,
      }),
    ).rejects.toThrow(/Insufficient token balance/);
    expect(sendTransaction).not.toHaveBeenCalled();
  });

  it("uses custom contract addresses if provided (ETH)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    const customAddress = "0x9999999999999999999999999999999999999999" as Address;
    await deposit(client, {
      l2Client,
      token: zeroAddress,
      to,
      amount,
      data,
      account: mockAccount,
      lineaRollupAddress: customAddress,
      l2MessageServiceAddress: customAddress,
    });
    expect(sendTransaction).toHaveBeenCalledTimes(1);
    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: customAddress,
        data: expect.any(String),
      }),
    );
  });

  it("uses custom contract addresses if provided (ERC20)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    const customL1 = "0x8888888888888888888888888888888888888888" as Address;
    const customL2 = "0x7777777777777777777777777777777777777777" as Address;
    (multicall as jest.Mock).mockResolvedValueOnce([
      {
        result: "TokenName",
        status: "success",
      },
      {
        result: "TKN",
        status: "success",
      },
      {
        result: 18,
        status: "success",
      },
      {
        result: "0x1234567890123456789012345678901234567890", // bridgedToken
        status: "success",
      },
    ]);
    (multicall as jest.Mock).mockResolvedValueOnce([200n, 200n]);
    await deposit(client, {
      l2Client,
      token,
      to,
      amount,
      data,
      account: mockAccount,
      l1TokenBridgeAddress: customL1,
      l2TokenBridgeAddress: customL2,
    });
    expect(sendTransaction).toHaveBeenCalledTimes(1);
    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: customL1,
        data: expect.any(String),
      }),
    );
  });

  it("handles bridged token scenario (nativeToken !== zeroAddress)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (multicall as jest.Mock<ReturnType<typeof multicall>>).mockResolvedValueOnce([
      {
        result: "TokenName",
        status: "success",
      },
      {
        result: "TKN",
        status: "success",
      },
      {
        result: 18,
        status: "success",
      },
      {
        result: "0x1234567890123456789012345678901234567890", // bridgedToken
        status: "success",
      },
    ]);
    (multicall as jest.Mock).mockResolvedValueOnce([200n, 200n]);
    await deposit(client, {
      l2Client,
      token,
      to,
      amount,
      data,
      account: mockAccount,
    });
    expect(multicall).toHaveBeenCalledTimes(2);
    expect(sendTransaction).toHaveBeenCalledTimes(1);
  });

  it("defaults data to '0x' if not provided (ETH)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    await deposit(client, {
      l2Client,
      token: zeroAddress,
      to,
      amount,
      account: mockAccount,
    });
    expect(sendTransaction).toHaveBeenCalledTimes(1);
    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
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
          args: [to, 693049000n, "0x"],
        }),
      }),
    );
  });

  it("propagates errors from multicall (ERC20)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (multicall as jest.Mock<ReturnType<typeof multicall>>)
      .mockResolvedValueOnce([
        {
          result: "TokenName",
          status: "success",
        },
        {
          result: "TKN",
          status: "success",
        },
        {
          result: 18,
          status: "success",
        },
        {
          result: TEST_ADDRESS_1,
          status: "success",
        },
      ])
      .mockRejectedValueOnce(new Error("multicall failed"));
    await expect(
      deposit(client, {
        l2Client,
        token,
        to,
        amount,
        data,
        account: mockAccount,
      }),
    ).rejects.toThrow("multicall failed");
    expect(sendTransaction).not.toHaveBeenCalled();
  });

  it("propagates errors from sendTransaction (ETH)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (sendTransaction as jest.Mock).mockRejectedValueOnce(new Error("sendTransaction failed"));
    await expect(
      deposit(client, {
        l2Client,
        token: zeroAddress,
        to,
        amount,
        data,
        account: mockAccount,
      }),
    ).rejects.toThrow("sendTransaction failed");
    expect(sendTransaction).toHaveBeenCalledTimes(1);
  });

  it("propagates errors from estimateFeesPerGas (ETH)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (estimateFeesPerGas as jest.Mock).mockRejectedValueOnce(new Error("estimateFeesPerGas failed"));
    await expect(
      deposit(client, {
        l2Client,
        token: zeroAddress,
        to,
        amount,
        data,
        account: mockAccount,
      }),
    ).rejects.toThrow("estimateFeesPerGas failed");
    expect(sendTransaction).not.toHaveBeenCalled();
  });

  it("propagates errors from estimateContractGas (ETH)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (estimateContractGas as jest.Mock).mockRejectedValueOnce(new Error("estimateContractGas failed"));
    await expect(
      deposit(client, {
        l2Client,
        token: zeroAddress,
        to,
        amount,
        data,
        account: mockAccount,
      }),
    ).rejects.toThrow("estimateContractGas failed");
    expect(sendTransaction).not.toHaveBeenCalled();
  });

  it("throws if tokenDecimalsResult.status is not 'success' (ERC20)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (multicall as jest.Mock).mockResolvedValueOnce([
      { result: "TokenName", status: "success" },
      { result: "TKN", status: "success" },
      { status: "failure", error: "decimals error" },
      { result: zeroAddress, status: "success" },
    ]);
    await expect(
      deposit(client, {
        l2Client,
        token,
        to,
        amount,
        data,
        account: mockAccount,
      }),
    ).rejects.toThrow(`Failed to fetch token decimals for ${token}. Error: decimals error`);
    expect(sendTransaction).not.toHaveBeenCalled();
  });

  it("throws if nativeTokenResult.status is not 'success' (ERC20)", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (multicall as jest.Mock).mockResolvedValueOnce([
      { result: "TokenName", status: "success" },
      { result: "TKN", status: "success" },
      { result: 18, status: "success" },
      { status: "failure", error: "native token error" },
    ]);
    await expect(
      deposit(client, {
        l2Client,
        token,
        to,
        amount,
        data,
        account: mockAccount,
      }),
    ).rejects.toThrow(`Failed to fetch native token for ${token}. Error: native token error`);
    expect(sendTransaction).not.toHaveBeenCalled();
  });

  it("uses fallback values for tokenName and tokenSymbol if multicall fails", async () => {
    const client = mockClient(l1ChainId, mockAccount);
    const l2Client = mockL2Client(l2ChainId, mockAccount);
    (multicall as jest.Mock)
      .mockResolvedValueOnce([
        { status: "failure", error: "token name error" },
        { status: "failure", error: "token symbol" },
        { status: "success", result: 18 },
        { status: "success", result: zeroAddress },
      ])
      .mockResolvedValueOnce([200n, 200n]);

    const result = await deposit(client, {
      l2Client,
      token,
      to,
      amount,
      data,
      account: mockAccount,
    });

    expect(sendTransaction).toHaveBeenCalledWith(
      client,
      expect.objectContaining({
        to: l1TokenBridgeAddress,
        value: 693049000n,
        account: mockAccount,
        data: expect.any(String),
      }),
    );
    expect(result).toBe(TEST_TRANSACTION_HASH);
  });
});

describe("deposit utility functions", () => {
  it("computeMessageHash returns correct hash", () => {
    const from = "0x1111111111111111111111111111111111111111" as Address;
    const to = "0x2222222222222222222222222222222222222222" as Address;
    const fee = 1n;
    const value = 2n;
    const nonce = 3n;
    const calldata = "0x" as Hex;

    const hash = computeMessageHash({ from, to, fee, value, nonce, calldata });
    expect(hash).toBe("0x1ebd3a6c6d29012c12e2e6cc8c9cc3346ccd756b4b997e2c435b1a8b4c7c00e7");
  });

  it("computeMessageHash uses default calldata value '0x' when not provided", () => {
    const from = TEST_ADDRESS_1 as Address;
    const toAddr = TEST_ADDRESS_2 as Address;
    const fee = 1n;
    const value = 2n;
    const nonce = 3n;

    const hash = computeMessageHash({ from, to: toAddr, fee, value, nonce });
    const hashWithCalldata = computeMessageHash({ from, to: toAddr, fee, value, nonce, calldata: "0x" });

    expect(hash).toBe(hashWithCalldata);
  });

  it("computeMessageStorageSlot returns correct slot", () => {
    const messageHash = "0x" + "a".repeat(64);
    const slot = computeMessageStorageSlot(messageHash as Hex);
    expect(slot).toBe("0xdaa0649102c4e0b3eba6c3d3634428c0e6bf46eb30a919f60194ee4cbaca2db3");
  });

  it("createStateOverride returns correct structure", () => {
    const address = "0x1111111111111111111111111111111111111111" as Address;
    const slot = ("0x" + "b".repeat(64)) as Hex;
    const override = createStateOverride(address, slot);
    expect(override).toStrictEqual([
      {
        address,
        stateDiff: [
          {
            slot,
            value: "0x0000000000000000000000000000000000000000000000000000000000000001" as Hex,
          },
        ],
      },
    ]);
  });
});
