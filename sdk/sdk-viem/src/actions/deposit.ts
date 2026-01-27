import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";
import {
  Account,
  Address,
  BaseError,
  Chain,
  ChainNotFoundError,
  ChainNotFoundErrorType,
  Client,
  ClientChainNotConfiguredError,
  ClientChainNotConfiguredErrorType,
  DeriveChain,
  encodeAbiParameters,
  EncodeAbiParametersErrorType,
  encodeFunctionData,
  EncodeFunctionDataErrorType,
  erc20Abi,
  EstimateContractGasErrorType,
  EstimateContractGasParameters,
  EstimateFeesPerGasErrorType,
  FormattedTransactionRequest,
  GetBlockErrorType,
  GetChainParameter,
  Hex,
  keccak256,
  MulticallErrorType,
  SendTransactionErrorType,
  SendTransactionParameters,
  SendTransactionReturnType,
  StateOverride,
  Transport,
  WaitForTransactionReceiptErrorType,
  zeroAddress,
} from "viem";
import {
  estimateContractGas,
  estimateFeesPerGas,
  getBlock,
  multicall,
  sendTransaction,
  waitForTransactionReceipt,
} from "viem/actions";
import { parseAccount } from "viem/utils";

import { AccountNotFoundError, AccountNotFoundErrorType } from "../errors/account";
import { GetAccountParameter } from "../types/account";
import { computeMessageHash } from "../utils/computeMessageHash";
import { getNextMessageNonce, GetNextMessageNonceErrorType } from "../utils/getNextMessageNonce";

export type DepositParameters<
  chain extends Chain | undefined = Chain | undefined,
  account extends Account | undefined = Account | undefined,
  chainOverride extends Chain | undefined = Chain | undefined,
  chainL2 extends Chain | undefined = Chain | undefined,
  accountL2 extends Account | undefined = Account | undefined,
  derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
> = Omit<FormattedTransactionRequest<derivedChain>, "data" | "to" | "from"> &
  Partial<GetChainParameter<chain, chainOverride>> &
  Partial<GetAccountParameter<account>> & {
    l2Client: Client<Transport, chainL2, accountL2>;
    token: Address;
    to: Address;
    fee?: bigint;
    amount: bigint;
    data?: Hex;
    // defaults to the message service address for the chain
    lineaRollupAddress?: Address;
    // defaults to the L2 message service address for the chain
    l2MessageServiceAddress?: Address;
    // defaults to the L1 token bridge address for the chain
    l1TokenBridgeAddress?: Address;
    // defaults to the L2 token bridge address for the chain
    l2TokenBridgeAddress?: Address;
  };

export type DepositReturnType = SendTransactionReturnType;

export type DepositErrorType =
  | SendTransactionErrorType
  | EstimateContractGasErrorType
  | EstimateFeesPerGasErrorType
  | GetNextMessageNonceErrorType
  | MulticallErrorType
  | WaitForTransactionReceiptErrorType
  | GetBlockErrorType
  | EncodeFunctionDataErrorType
  | EncodeAbiParametersErrorType
  | ChainNotFoundErrorType
  | ClientChainNotConfiguredErrorType
  | AccountNotFoundErrorType;

/**
 * Deposits tokens from L1 to L2 or ETH if `token` is set to `zeroAddress`.
 *
 * @param client - Client to use
 * @param parameters - {@link DepositParameters}
 * @returns hash - The [Transaction](https://viem.sh/docs/glossary/terms#transaction) hash. {@link DepositReturnType}
 *
 * @example
 * import { createWalletClient, http, zeroAddress } from 'viem'
 * import { privateKeyToAccount } from 'viem/accounts'
 * import { mainnet, linea } from 'viem/chains'
 * import { deposit } from '@consensys/linea-sdk-viem'
 *
 * const client = createWalletClient({
 *   chain: mainnet,
 *   transport: http(),
 * });
 *
 * const l2Client = createPublicClient({
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const hash = await deposit(client, {
 *     l2Client,
 *     account: privateKeyToAccount('0x…'),
 *     amount: 1_000_000_000_000n,
 *     token: zeroAddress, // Use zeroAddress for ETH
 *     to: '0xRecipientAddress',
 *     data: '0x', // Optional data
 *     fee: 100_000_000n, // Optional fee
 * });
 *
 * @example Account Hoisting
 * import { createPublicClient, createWalletClient, http, zeroAddress } from 'viem'
 * import { privateKeyToAccount } from 'viem/accounts'
 * import { mainnet, linea } from 'viem/chains'
 * import { deposit } from '@consensys/linea-sdk-viem'
 *
 * const client = createWalletClient({
 *   account: privateKeyToAccount('0x…'),
 *   chain: mainnet,
 *   transport: http(),
 * });
 *
 * const l2Client = createPublicClient({
 *  chain: linea,
 *  transport: http(),
 * });
 *
 * const hash = await deposit(client, {
 *     l2Client
 *     amount: 1_000_000_000_000n,
 *     token: zeroAddress, // Use zeroAddress for ETH
 *     to: '0xRecipientAddress',
 *     data: '0x', // Optional data
 *     fee: 100_000_000n, // Optional fee
 * });
 */

export async function deposit<
  chain extends Chain | undefined,
  account extends Account | undefined,
  chainOverride extends Chain | undefined = Chain | undefined,
  chainL2 extends Chain | undefined = Chain | undefined,
  accountL2 extends Account | undefined = Account | undefined,
  derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
>(
  client: Client<Transport, chain, account>,
  parameters: DepositParameters<chain, account, chainOverride, chainL2, accountL2, derivedChain>,
): Promise<DepositReturnType> {
  const { account: account_ = client.account, l2Client, token, amount, data, to, fee, ...tx } = parameters;

  const account = account_ ? parseAccount(account_) : client.account;
  if (!account) {
    throw new AccountNotFoundError({
      docsPath: "/docs/actions/wallet/sendTransaction",
    });
  }

  if (!client.chain) {
    throw new ChainNotFoundError();
  }

  if (!l2Client.chain) {
    throw new ClientChainNotConfiguredError();
  }

  const l1ChainId = client.chain.id;
  const l2ChainId = l2Client.chain.id;

  const lineaRollupAddress = parameters.lineaRollupAddress ?? getContractsAddressesByChainId(l1ChainId).messageService;
  const l2MessageServiceAddress =
    parameters.l2MessageServiceAddress ?? getContractsAddressesByChainId(l2ChainId).messageService;
  const l1TokenBridgeAddress = parameters.l1TokenBridgeAddress ?? getContractsAddressesByChainId(l1ChainId).tokenBridge;
  const l2TokenBridgeAddress = parameters.l2TokenBridgeAddress ?? getContractsAddressesByChainId(l2ChainId).tokenBridge;

  if (token === zeroAddress) {
    return depositETH(client, {
      l2Client,
      account,
      lineaRollupAddress,
      l2MessageServiceAddress,
      to: to as Address,
      fee,
      amount,
      data: data ?? "0x",
      tx,
    });
  }

  return depositERC20(client, {
    l2Client,
    account,
    lineaRollupAddress,
    l2MessageServiceAddress,
    l1TokenBridgeAddress,
    l2TokenBridgeAddress,
    l1ChainId,
    l2ChainId,
    token: token as Address,
    to: to as Address,
    fee,
    amount,
    tx,
  });
}

async function estimateEthBridgingGasUsed<chain extends Chain | undefined, _account extends Account | undefined>(
  client: Client<Transport, chain, _account>,
  parameters: {
    account: Address;
    recipient: Address;
    amount: bigint;
    nextMessageNonce: bigint;
    l2MessageServiceAddress: Address; // Optional, defaults to the message service address for the chain
  },
) {
  const { account, recipient, amount, nextMessageNonce, l2MessageServiceAddress } = parameters;
  const messageHash = computeMessageHash({
    from: account,
    to: recipient,
    fee: 0n,
    value: amount,
    nonce: nextMessageNonce,
    calldata: "0x",
  });

  const storageSlot = computeMessageStorageSlot(messageHash);
  const stateOverride = createStateOverride(l2MessageServiceAddress, storageSlot);

  return estimateContractGas(client, {
    address: l2MessageServiceAddress,
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
    ] as const,
    functionName: "claimMessage",
    account: account as `0x${string}`,
    args: [account, recipient, 0n, amount, zeroAddress as `0x${string}`, "0x" as `0x${string}`, nextMessageNonce],
    stateOverride,
  } as EstimateContractGasParameters);
}

async function estimateERC20BridgingGasUsed<
  chain extends Chain | undefined,
  _account extends Account | undefined,
  chainL2 extends Chain | undefined = Chain | undefined,
  accountL2 extends Account | undefined = Account | undefined,
>(
  client: Client<Transport, chain, _account>,
  parameters: {
    l1Client: Client<Transport, chainL2, accountL2>;
    account: Address;
    token: Address;
    l1ChainId: number;
    l2ChainId: number;
    amount: bigint;
    recipient: Address;
    nextMessageNonce: bigint;
    l2MessageServiceAddress: Address;
    l1TokenBridgeAddress: Address;
    l2TokenBridgeAddress: Address;
  },
) {
  const {
    l1Client,
    token,
    l1ChainId,
    l2ChainId,
    amount,
    recipient,
    nextMessageNonce,
    account,
    l2MessageServiceAddress,
    l1TokenBridgeAddress,
    l2TokenBridgeAddress,
  } = parameters;

  const { tokenAddress, chainId, tokenMetadata } = await prepareERC20TokenParams(l1Client, {
    token,
    l1ChainId,
    l2ChainId,
  });

  const encodedData = encodeFunctionData({
    abi: [
      {
        inputs: [
          {
            internalType: "address",
            name: "_nativeToken",
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
          {
            internalType: "uint256",
            name: "_chainId",
            type: "uint256",
          },
          {
            internalType: "bytes",
            name: "_tokenMetadata",
            type: "bytes",
          },
        ],
        name: "completeBridging",
        outputs: [],
        stateMutability: "nonpayable",
        type: "function",
      },
    ],
    functionName: "completeBridging",
    args: [tokenAddress, amount, recipient, BigInt(chainId), tokenMetadata],
  });

  const messageHash = computeMessageHash({
    from: l1TokenBridgeAddress,
    to: l2TokenBridgeAddress,
    fee: 0n,
    value: 0n,
    nonce: nextMessageNonce,
    calldata: encodedData,
  });

  const storageSlot = computeMessageStorageSlot(messageHash);
  const stateOverride = createStateOverride(l2MessageServiceAddress, storageSlot);

  return estimateContractGas(client, {
    address: l2MessageServiceAddress,
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
    ] as const,
    functionName: "claimMessage",
    account: account,
    args: [l1TokenBridgeAddress, l2TokenBridgeAddress, 0n, 0n, zeroAddress, encodedData, nextMessageNonce],
    stateOverride,
  } as EstimateContractGasParameters);
}

async function prepareERC20TokenParams<chain extends Chain | undefined, _account extends Account | undefined>(
  client: Client<Transport, chain, _account>,
  parameters: {
    token: Address;
    l1ChainId: number;
    l2ChainId: number;
  },
): Promise<{ tokenAddress: Address; chainId: number; tokenMetadata: Hex }> {
  const { token, l1ChainId, l2ChainId } = parameters;

  const [tokenNameResult, tokenSymbolResult, tokenDecimalsResult, nativeTokenResult] = await multicall(client, {
    contracts: [
      {
        address: token,
        abi: erc20Abi,
        functionName: "name",
      },
      {
        address: token,
        abi: erc20Abi,
        functionName: "symbol",
      },
      {
        address: token,
        abi: erc20Abi,
        functionName: "decimals",
      },
      {
        address: getContractsAddressesByChainId(l1ChainId).tokenBridge,
        abi: [
          {
            inputs: [
              {
                internalType: "address",
                name: "bridged",
                type: "address",
              },
            ],
            name: "bridgedToNativeToken",
            outputs: [
              {
                internalType: "address",
                name: "native",
                type: "address",
              },
            ],
            stateMutability: "view",
            type: "function",
          },
        ],
        functionName: "bridgedToNativeToken",
        args: [token],
      },
    ],
    allowFailure: true,
  });

  const tokenName = tokenNameResult.status === "success" ? tokenNameResult.result : "NO_NAME";
  const tokenSymbol = tokenSymbolResult.status === "success" ? tokenSymbolResult.result : "NO_SYMBOL";

  if (tokenDecimalsResult.status !== "success") {
    throw new BaseError(`Failed to fetch token decimals for ${token}. Error: ${tokenDecimalsResult.error}`);
  }

  if (nativeTokenResult.status !== "success") {
    throw new BaseError(`Failed to fetch native token for ${token}. Error: ${nativeTokenResult.error}`);
  }

  let tokenAddress = token;
  let chainId = l1ChainId;

  let tokenMetadata = encodeAbiParameters(
    [
      { name: "tokenName", type: "string" },
      { name: "tokenSymbol", type: "string" },
      { name: "tokenDecimals", type: "uint8" },
    ],
    [tokenName, tokenSymbol, tokenDecimalsResult.result],
  );

  if (nativeTokenResult.result !== zeroAddress) {
    tokenAddress = nativeTokenResult.result;
    chainId = l2ChainId;
    tokenMetadata = "0x";
  }

  return { tokenAddress, chainId, tokenMetadata };
}

export function computeMessageStorageSlot(messageHash: `0x${string}`) {
  return keccak256(
    encodeAbiParameters(
      [
        { name: "messageHash", type: "bytes32" },
        { name: "mappingSlot", type: "uint256" },
      ],
      [messageHash, 176n],
    ),
  );
}

export function createStateOverride(messageServiceAddress: Address, storageSlot: Hex): StateOverride {
  return [
    {
      address: messageServiceAddress,
      stateDiff: [
        {
          slot: storageSlot,
          value: "0x0000000000000000000000000000000000000000000000000000000000000001" as Hex,
        },
      ],
    },
  ];
}

async function depositETH<
  chain extends Chain | undefined,
  account extends Account | undefined,
  chainOverride extends Chain | undefined = Chain | undefined,
  chainL2 extends Chain | undefined = Chain | undefined,
  accountL2 extends Account | undefined = Account | undefined,
  derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
>(
  client: Client<Transport, chain, account>,
  parameters: {
    l2Client: Client<Transport, chainL2, accountL2>;
    account: Account;
    lineaRollupAddress: Address;
    l2MessageServiceAddress: Address;
    to: Address;
    fee: bigint | undefined;
    amount: bigint;
    data: Hex;
    tx: Omit<
      DepositParameters<chain, account, chainOverride, chainL2, accountL2, derivedChain>,
      "account" | "to" | "data" | "amount" | "token" | "l2Client" | "fee"
    >;
  },
): Promise<DepositReturnType> {
  const {
    l2Client,
    account: account_,
    lineaRollupAddress,
    l2MessageServiceAddress,
    amount,
    to,
    data,
    fee,
    tx,
  } = parameters;

  let bridgingFee = fee ?? 0n;

  if (fee === undefined) {
    const [nextMessageNonce, { baseFeePerGas }, { maxPriorityFeePerGas }] = await Promise.all([
      getNextMessageNonce(client, {
        lineaRollupAddress,
      }),
      getBlock(l2Client, { blockTag: "latest" }),
      estimateFeesPerGas(l2Client, { type: "eip1559", chain: l2Client.chain }),
    ]);

    const l2ClaimingTxGasLimit = await estimateEthBridgingGasUsed(l2Client, {
      account: account_.address,
      recipient: to as Address,
      amount,
      nextMessageNonce,
      l2MessageServiceAddress,
    });
    bridgingFee = (baseFeePerGas + maxPriorityFeePerGas) * (l2ClaimingTxGasLimit + 6_000n);
  }

  return sendTransaction(client, {
    to: lineaRollupAddress,
    value: amount + bridgingFee,
    account: account_,
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
      args: [to, bridgingFee, data],
    }),
    ...tx,
  } as SendTransactionParameters);
}

async function depositERC20<
  chain extends Chain | undefined,
  account extends Account | undefined,
  chainOverride extends Chain | undefined = Chain | undefined,
  chainL2 extends Chain | undefined = Chain | undefined,
  accountL2 extends Account | undefined = Account | undefined,
  derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
>(
  client: Client<Transport, chain, account>,
  parameters: {
    l2Client: Client<Transport, chainL2, accountL2>;
    account: Account;
    lineaRollupAddress: Address;
    l2MessageServiceAddress: Address;
    l1TokenBridgeAddress: Address;
    l2TokenBridgeAddress: Address;
    l1ChainId: number;
    l2ChainId: number;
    token: Address;
    to: Address;
    fee: bigint | undefined;
    amount: bigint;
    tx: Omit<
      DepositParameters<chain, account, chainOverride, chainL2, accountL2, derivedChain>,
      "account" | "to" | "data" | "amount" | "token" | "l2Client" | "fee"
    >;
  },
): Promise<DepositReturnType> {
  const {
    l2Client,
    account,
    l1TokenBridgeAddress,
    l2TokenBridgeAddress,
    lineaRollupAddress,
    l2MessageServiceAddress,
    l1ChainId,
    l2ChainId,
    token,
    to,
    fee,
    amount,
    tx,
  } = parameters;

  let bridgingFee = fee ?? 0n;

  if (fee === undefined) {
    const [nextMessageNonce, { baseFeePerGas }, { maxPriorityFeePerGas }] = await Promise.all([
      getNextMessageNonce(client, {
        lineaRollupAddress,
      }),
      getBlock(l2Client, { blockTag: "latest" }),
      estimateFeesPerGas(l2Client, { type: "eip1559", chain: l2Client.chain }),
    ]);

    const l2ClaimingTxGasLimit = await estimateERC20BridgingGasUsed(l2Client, {
      l1Client: client,
      account: account.address,
      token,
      l1ChainId,
      l2ChainId,
      amount,
      recipient: to,
      nextMessageNonce,
      l2MessageServiceAddress,
      l1TokenBridgeAddress,
      l2TokenBridgeAddress,
    });
    bridgingFee = (baseFeePerGas + maxPriorityFeePerGas) * (l2ClaimingTxGasLimit + 6_000n);
  }

  const [tokenBalance, allowance] = await multicall(client, {
    contracts: [
      {
        address: token,
        abi: erc20Abi,
        functionName: "balanceOf",
        args: [account.address],
      },
      {
        address: token,
        abi: erc20Abi,
        functionName: "allowance",
        args: [account.address, l1TokenBridgeAddress],
      },
    ],
    allowFailure: false,
  });

  if (tokenBalance < amount) {
    throw new BaseError(
      `Insufficient token balance for bridging. Current balance: ${tokenBalance}, required: ${amount}`,
    );
  }

  if (allowance < amount) {
    const approveTxHash = await sendTransaction(client, {
      to: token,
      account: account,
      data: encodeFunctionData({
        abi: erc20Abi,
        functionName: "approve",
        args: [l1TokenBridgeAddress, amount],
      }),
    } as SendTransactionParameters);

    await waitForTransactionReceipt(client, {
      hash: approveTxHash,
    });
  }

  return sendTransaction(client, {
    to: l1TokenBridgeAddress,
    value: bridgingFee,
    account: account,
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
    ...tx,
  } as SendTransactionParameters);
}
