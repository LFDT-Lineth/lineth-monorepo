import { getContractsAddressesByChainId, Message } from "@consensys/linea-sdk-core";
import {
  Account,
  Address,
  Chain,
  ChainNotFoundError,
  ChainNotFoundErrorType,
  Client,
  DeriveChain,
  encodeFunctionData,
  FormattedTransactionRequest,
  GetChainParameter,
  SendTransactionErrorType,
  SendTransactionParameters,
  SendTransactionReturnType,
  Transport,
  zeroAddress,
} from "viem";
import { sendTransaction } from "viem/actions";
import { parseAccount } from "viem/utils";

import { AccountNotFoundError, AccountNotFoundErrorType } from "../errors/account";
import { GetAccountParameter } from "../types/account";

export type ClaimOnL2Parameters<
  chain extends Chain | undefined = Chain | undefined,
  account extends Account | undefined = Account | undefined,
  chainOverride extends Chain | undefined = Chain | undefined,
  derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
> = Omit<FormattedTransactionRequest<derivedChain>, "data" | "to" | "from"> &
  Partial<GetChainParameter<chain, chainOverride>> &
  Partial<GetAccountParameter<account>> &
  Omit<Message, "messageHash" | "nonce"> & {
    messageNonce: bigint;
    feeRecipient?: Address;
    // defaults to the message service address for the L2 chain
    l2MessageServiceAddress?: Address;
  };

export type ClaimOnL2ReturnType = SendTransactionReturnType;

export type ClaimOnL2ErrorType = SendTransactionErrorType | ChainNotFoundErrorType | AccountNotFoundErrorType;

/**
 * Claim a message on L2.
 *
 * @param client - Client to use
 * @param parameters - {@link ClaimOnL2Parameters}
 * @returns hash - The [Transaction](https://viem.sh/docs/glossary/terms#transaction) hash. {@link ClaimOnL2ReturnType}
 *
 * @example
 * import { createWalletClient, http, zeroAddress } from 'viem'
 * import { privateKeyToAccount } from 'viem/accounts'
 * import { linea } from 'viem/chains'
 * import { claimOnL2 } from '@consensys/linea-sdk-viem'
 *
 * const client = createWalletClient({
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const hash = await claimOnL2(client, {
 *     account: privateKeyToAccount('0x…'),
 *     from: '0xSenderAddress',
 *     to: '0xRecipientAddress',
 *     fee: 100_000_000n, // Fee in wei
 *     value: 1_000_000_000_000n, // Amount in wei
 *     messageNonce: 1n, // Nonce of the message to claim
 *     calldata: '0x',
 *     feeRecipient: zeroAddress, // Optional fee recipient, defaults to zeroAddress
 *     // Optional transaction parameters
 *     gas: 21000n, // Gas limit
 *     maxFeePerGas: 100_000_000n, // Max fee per gas
 *     maxPriorityFeePerGas: 1_000_000n, // Max priority fee per gas
 * });
 *
 * @example Account Hoisting
 * import { createWalletClient, http, zeroAddress } from 'viem'
 * import { privateKeyToAccount } from 'viem/accounts'
 * import { linea } from 'viem/chains'
 * import { claimOnL2 } from '@consensys/linea-sdk-viem'
 *
 * const client = createWalletClient({
 *   account: privateKeyToAccount('0x…'),
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const hash = await claimOnL2(client, {
 *     from: '0xSenderAddress',
 *     to: '0xRecipientAddress',
 *     fee: 100_000_000n, // Fee in wei
 *     value: 1_000_000_000_000n, // Amount in wei
 *     messageNonce: 1n, // Nonce of the message to claim
 *     calldata: '0x',
 *     feeRecipient: zeroAddress, // Optional fee recipient, defaults to zeroAddress
 *     // Optional transaction parameters
 *     gas: 21000n, // Gas limit
 *     maxFeePerGas: 100_000_000n, // Max fee per gas
 *     maxPriorityFeePerGas: 1_000_000n, // Max priority fee per gas
 * });
 */
export async function claimOnL2<
  chain extends Chain | undefined,
  account extends Account | undefined,
  chainOverride extends Chain | undefined = Chain | undefined,
  derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
>(
  client: Client<Transport, chain, account>,
  parameters: ClaimOnL2Parameters<chain, account, chainOverride, derivedChain>,
): Promise<ClaimOnL2ReturnType> {
  const {
    account: account_ = client.account,
    from,
    to,
    fee,
    value,
    messageNonce,
    calldata,
    feeRecipient,
    ...tx
  } = parameters;

  const account = account_ ? parseAccount(account_) : client.account;
  if (!account) {
    throw new AccountNotFoundError({
      docsPath: "/docs/actions/wallet/sendTransaction",
    });
  }

  if (!client.chain) {
    throw new ChainNotFoundError();
  }

  const l2MessageServiceAddress =
    parameters.l2MessageServiceAddress ?? getContractsAddressesByChainId(client.chain.id).messageService;

  return sendTransaction(client, {
    to: l2MessageServiceAddress,
    account,
    data: encodeFunctionData({
      abi: [
        {
          inputs: [
            {
              internalType: "address",
              name: "_from",
              type: "address",
            },
            {
              internalType: "address",
              name: "_to",
              type: "address",
            },
            {
              internalType: "uint256",
              name: "_fee",
              type: "uint256",
            },
            {
              internalType: "uint256",
              name: "_value",
              type: "uint256",
            },
            {
              internalType: "address payable",
              name: "_feeRecipient",
              type: "address",
            },
            {
              internalType: "bytes",
              name: "_calldata",
              type: "bytes",
            },
            {
              internalType: "uint256",
              name: "_nonce",
              type: "uint256",
            },
          ],
          name: "claimMessage",
          outputs: [],
          stateMutability: "nonpayable",
          type: "function",
        },
      ],
      functionName: "claimMessage",
      args: [from, to, fee, value, feeRecipient ?? zeroAddress, calldata, messageNonce],
    }),
    ...tx,
  } as SendTransactionParameters);
}
