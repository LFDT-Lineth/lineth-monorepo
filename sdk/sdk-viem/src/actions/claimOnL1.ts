import {
  Account,
  Address,
  Chain,
  ChainNotFoundError,
  ChainNotFoundErrorType,
  Client,
  ClientChainNotConfiguredError,
  ClientChainNotConfiguredErrorType,
  DeriveChain,
  encodeFunctionData,
  FormattedTransactionRequest,
  GetChainParameter,
  OneOf,
  SendTransactionErrorType,
  SendTransactionParameters,
  SendTransactionReturnType,
  Transport,
  zeroAddress,
} from "viem";
import { GetAccountParameter } from "../types/account";
import { parseAccount } from "viem/utils";
import { sendTransaction } from "viem/actions";
import { getContractsAddressesByChainId, MessageProof, Message } from "@consensys/linea-sdk-core";
import { getMessageProof } from "./getMessageProof";
import { computeMessageHash, ComputeMessageHashErrorType } from "../utils/computeMessageHash";
import { AccountNotFoundError, AccountNotFoundErrorType } from "../errors/account";
import {
  MissingMessageProofOrClientForClaimingOnL1Error,
  MissingMessageProofOrClientForClaimingOnL1ErrorType,
} from "../errors/bridge";

export type ClaimOnL1Parameters<
  chain extends Chain | undefined = Chain | undefined,
  account extends Account | undefined = Account | undefined,
  chainL2 extends Chain | undefined = Chain | undefined,
  accountL2 extends Account | undefined = Account | undefined,
  chainOverride extends Chain | undefined = Chain | undefined,
  derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
> = Omit<FormattedTransactionRequest<derivedChain>, "data" | "to" | "from"> &
  Partial<GetChainParameter<chain, chainOverride>> &
  Partial<GetAccountParameter<account>> &
  Omit<Message<bigint>, "messageHash" | "nonce"> &
  OneOf<
    | {
        l2Client: Client<Transport, chainL2, accountL2>;
        messageNonce: bigint;
        feeRecipient?: Address;
        // defaults to the message service address for the L1 chain
        lineaRollupAddress?: Address;
        // Defaults to the message service address for the L2 chain
        l2MessageServiceAddress?: Address;
      }
    | {
        messageNonce: bigint;
        messageProof: MessageProof;
        feeRecipient?: Address;
        // defaults to the message service address for the L1 chain
        lineaRollupAddress?: Address;
      }
  >;

export type ClaimOnL1ReturnType = SendTransactionReturnType;

export type ClaimOnL1ErrorType =
  | SendTransactionErrorType
  | ChainNotFoundErrorType
  | ClientChainNotConfiguredErrorType
  | ComputeMessageHashErrorType
  | AccountNotFoundErrorType
  | MissingMessageProofOrClientForClaimingOnL1ErrorType;

/**
 * Claim a message on L1.
 *
 * @param client - Client to use
 * @param parameters - {@link ClaimOnL1Parameters}
 * @returns hash - The [Transaction](https://viem.sh/docs/glossary/terms#transaction) hash. {@link ClaimOnL1ReturnType}
 *
 * @example
 * import { createWalletClient, http, zeroAddress } from 'viem'
 * import { privateKeyToAccount } from 'viem/accounts'
 * import { mainnet } from 'viem/chains'
 * import { claimOnL1 } from '@consensys/linea-sdk-viem'
 *
 * const client = createWalletClient({
 *   chain: mainnet,
 *   transport: http(),
 * });
 *
 * const hash = await claimOnL1(client, {
 *     account: privateKeyToAccount('0x…'),
 *     from: '0xSenderAddress',
 *     to: '0xRecipientAddress',
 *     fee: 100_000_000n, // Fee in wei
 *     value: 1_000_000_000_000n, // Amount in wei
 *     messageNonce: 1n, // Nonce of the message to claim
 *     calldata: '0x',
 *     feeRecipient: zeroAddress, // Optional fee recipient, defaults to zeroAddress
 *      messageProof: {
 *         root: '0x…', // Merkle root of the message
 *         proof: ['0x…'], // Merkle proof of the message
 *         leafIndex: 0, // Index of the leaf in the Merkle tree
 *     },
 *     // Optional transaction parameters
 *     gas: 21000n, // Gas limit
 *     maxFeePerGas: 100_000_000n, // Max fee per gas
 *     maxPriorityFeePerGas: 1_000_000n, // Max priority fee per gas
 * });
 *
 * @example Without providing messageProof
 * import { createWalletClient, http, zeroAddress } from 'viem'
 * import { privateKeyToAccount } from 'viem/accounts'
 * import { mainnet } from 'viem/chains'
 * import { claimOnL1 } from '@consensys/linea-sdk-viem'
 *
 * const client = createWalletClient({
 *   chain: mainnet,
 *   transport: http(),
 * });
 *
 * const l2Client = createPublicClient({
 *  chain: linea,
 *  transport: http(),
 * });
 *
 * const hash = await claimOnL1(client, {
 *     account: privateKeyToAccount('0x…'),
 *     from: '0xSenderAddress',
 *     to: '0xRecipientAddress',
 *     fee: 100_000_000n, // Fee in wei
 *     value: 1_000_000_000_000n, // Amount in wei
 *     messageNonce: 1n, // Nonce of the message to claim
 *     calldata: '0x',
 *     feeRecipient: zeroAddress, // Optional fee recipient, defaults to zeroAddress
 *     l2Client,
 *     // Optional transaction parameters
 *     gas: 21000n, // Gas limit
 *     maxFeePerGas: 100_000_000n, // Max fee per gas
 *     maxPriorityFeePerGas: 1_000_000n, // Max priority fee per gas
 * });
 *
 * @example Account Hoisting
 * import { createWalletClient, http, zeroAddress } from 'viem'
 * import { privateKeyToAccount } from 'viem/accounts'
 * import { mainnet } from 'viem/chains'
 * import { claimOnL1 } from '@consensys/linea-sdk-viem'
 *
 * const client = createWalletClient({
 *   account: privateKeyToAccount('0x…'),
 *   chain: mainnet,
 *   transport: http(),
 * });
 *
 * const hash = await claimOnL1(client, {
 *     from: '0xSenderAddress',
 *     to: '0xRecipientAddress',
 *     fee: 100_000_000n, // Fee in wei
 *     value: 1_000_000_000_000n, // Amount in wei
 *     messageNonce: 1n, // Nonce of the message to claim
 *     calldata: '0x',
 *     feeRecipient: zeroAddress, // Optional fee recipient, defaults to zeroAddress
 *      messageProof: {
 *         root: '0x…', // Merkle root of the message
 *         proof: ['0x…'], // Merkle proof of the message
 *         leafIndex: 0, // Index of the leaf in the Merkle tree
 *     },
 *     // Optional transaction parameters
 *     gas: 21000n, // Gas limit
 *     maxFeePerGas: 100_000_000n, // Max fee per gas
 *     maxPriorityFeePerGas: 1_000_000n, // Max priority fee per gas
 * });
 */
export async function claimOnL1<
  chain extends Chain | undefined = Chain | undefined,
  account extends Account | undefined = Account | undefined,
  chainL2 extends Chain | undefined = Chain | undefined,
  accountL2 extends Account | undefined = Account | undefined,
  chainOverride extends Chain | undefined = Chain | undefined,
  derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
>(
  client: Client<Transport, chain, account>,
  parameters: ClaimOnL1Parameters<chain, account, chainL2, accountL2, chainOverride, derivedChain>,
): Promise<ClaimOnL1ReturnType> {
  const {
    account: account_ = client.account,
    from,
    to,
    fee,
    value,
    messageNonce,
    calldata,
    feeRecipient,
    l2Client,
    messageProof,
    lineaRollupAddress,
    l2MessageServiceAddress,
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

  if (!messageProof && !l2Client) {
    throw new MissingMessageProofOrClientForClaimingOnL1Error();
  }

  if (l2Client && !l2Client.chain) {
    throw new ClientChainNotConfiguredError();
  }

  let proof = null;
  if (l2Client) {
    proof = await getMessageProof(client, {
      l2Client,
      lineaRollupAddress,
      l2MessageServiceAddress,
      messageHash: computeMessageHash({
        from,
        to,
        fee,
        value,
        nonce: messageNonce,
        calldata,
      }),
    });
  } else {
    proof = messageProof;
  }

  const lineaRollup = parameters.lineaRollupAddress ?? getContractsAddressesByChainId(client.chain.id).messageService;

  return sendTransaction(client, {
    to: lineaRollup,
    account,
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
          feeRecipient: feeRecipient ?? zeroAddress,
          data: calldata,
          messageNumber: messageNonce,
          merkleRoot: proof.root,
          proof: proof.proof,
          leafIndex: proof.leafIndex,
        },
      ],
    }),
    ...tx,
  } as SendTransactionParameters);
}
