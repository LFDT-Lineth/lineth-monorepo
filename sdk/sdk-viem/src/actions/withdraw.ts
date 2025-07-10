import {
  Account,
  Address,
  BaseError,
  Chain,
  Client,
  DeriveChain,
  encodeFunctionData,
  FormattedTransactionRequest,
  GetChainParameter,
  Hex,
  SendTransactionParameters,
  SendTransactionReturnType,
  Transport,
  zeroAddress,
} from "viem";
import { GetAccountParameter } from "../types/account";
import { parseAccount } from "viem/utils";
import { readContract, sendTransaction } from "viem/actions";
import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";

export type WithdrawParameters<
  chain extends Chain | undefined = Chain | undefined,
  account extends Account | undefined = Account | undefined,
  chainOverride extends Chain | undefined = Chain | undefined,
  derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
> = Omit<FormattedTransactionRequest<derivedChain>, "data" | "to" | "from"> &
  Partial<GetChainParameter<chain, chainOverride>> &
  Partial<GetAccountParameter<account>> & {
    token: Address;
    to: Address;
    amount: bigint;
    data?: Hex;
    /** defaults to the L2 message service address for the chain */
    l2MessageServiceAddress?: Address;
    /** defaults to the L2 token bridge address for the chain */
    l2TokenBridgeAddress?: Address;
  };

export type WithdrawReturnType = SendTransactionReturnType;

/**
 * Withdraws tokens from L2 to L1 or ETH if `token` is set to `zeroAddress`.
 *
 * @param client - Client to use
 * @param parameters - {@link WithdrawParameters}
 * @returns hash - The [Transaction](https://viem.sh/docs/glossary/terms#transaction) hash. {@link WithdrawReturnType}
 *
 * @example
 * import { createWalletClient, http, zeroAddress } from 'viem'
 * import { privateKeyToAccount } from 'viem/accounts'
 * import { linea } from 'viem/chains'
 * import { withdraw } from '@consensys/linea-sdk-viem'
 *
 * const client = createWalletClient({
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const hash = await withdraw(client, {
 *     account: privateKeyToAccount('0x…'),
 *     amount: 1_000_000_000_000n,
 *     token: zeroAddress, // Use zeroAddress for ETH
 *     to: '0xRecipientAddress',
 *     data: '0x', // Optional data
 * });
 *
 * @example Account Hoisting
 * import { createWalletClient, http, zeroAddress } from 'viem'
 * import { privateKeyToAccount } from 'viem/accounts'
 * import { linea } from 'viem/chains'
 * import { withdraw } from '@consensys/linea-sdk-viem'
 *
 * const client = createWalletClient({
 *   account: privateKeyToAccount('0x…'),
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const hash = await withdraw(client, {
 *     amount: 1_000_000_000_000n,
 *     token: zeroAddress, // Use zeroAddress for ETH
 *     to: '0xRecipientAddress',
 *     data: '0x', // Optional data
 * });
 */
export async function withdraw<
  chain extends Chain | undefined,
  account extends Account | undefined,
  chainOverride extends Chain | undefined = Chain | undefined,
  derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
>(
  client: Client<Transport, chain, account>,
  parameters: WithdrawParameters<chain, account, chainOverride, derivedChain>,
): Promise<WithdrawReturnType> {
  const { account: account_ = client.account, token, amount, to, data, ...tx } = parameters;

  const account = account_ ? parseAccount(account_) : client.account;

  if (!account) {
    throw new BaseError("Account is required to send a transaction");
  }

  const chainId = client.chain?.id;

  if (!chainId) {
    throw new BaseError("No chain id found");
  }

  const l2MessageServiceAddress =
    parameters.l2MessageServiceAddress ?? getContractsAddressesByChainId(chainId).messageService;

  const minimumFeeInWei = await readContract(client, {
    address: l2MessageServiceAddress,
    abi: [
      {
        inputs: [],
        name: "minimumFeeInWei",
        outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
        stateMutability: "view",
        type: "function",
      },
    ],
    functionName: "minimumFeeInWei",
  });

  if (token === zeroAddress) {
    return sendTransaction(client, {
      to: l2MessageServiceAddress,
      value: amount + minimumFeeInWei,
      account,
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
        args: [to, minimumFeeInWei, data ?? "0x"],
      }),
      ...tx,
    } as SendTransactionParameters);
  }

  const tokenBridgeAddress = parameters.l2TokenBridgeAddress ?? getContractsAddressesByChainId(chainId).tokenBridge;

  return sendTransaction(client, {
    to: tokenBridgeAddress,
    value: minimumFeeInWei,
    account,
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
