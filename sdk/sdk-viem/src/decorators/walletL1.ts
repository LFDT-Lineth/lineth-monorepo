import { Account, Address, Chain, Client, DeriveChain, Transport } from "viem";
import { L1WalletClient } from "@consensys/linea-sdk-core";
import { deposit, DepositParameters, DepositReturnType } from "../actions/deposit";
import { StrictFunctionOnly } from "../types/misc";
import { claimOnL1, ClaimOnL1Parameters, ClaimOnL1ReturnType } from "../actions/claimOnL1";

export type WalletActionsL1<
  chain extends Chain | undefined = Chain | undefined,
  account extends Account | undefined = Account | undefined,
> = StrictFunctionOnly<
  L1WalletClient,
  {
    /**
     * Deposits tokens from L1 to L2 or ETH if `token` is set to `zeroAddress`.
     *
     * @param args - {@link DepositParameters}
     * @returns hash - The [Transaction](https://viem.sh/docs/glossary/terms#transaction) hash. {@link DepositReturnType}
     *
     * @example
     * import { createWalletClient, http } from 'viem';
     * import { privateKeyToAccount } from 'viem/accounts'
     * import { mainnet, linea } from 'viem/chains'
     * import { walletActionsL1 } from '@consensys/linea-sdk-viem';
     *
     * const client = createWalletClient({
     *   chain: mainnet,
     *   transport: http(),
     * }).extend(walletActionsL1());
     *
     * const l2Client = createPublicClient({
     *   chain: linea,
     *   transport: http(),
     * });
     *
     * const hash = await client.deposit({
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
     * import { createWalletClient, http } from 'viem';
     * import { privateKeyToAccount } from 'viem/accounts';
     * import { mainnet, linea } from 'viem/chains';
     * import { walletActionsL1 } from '@consensys/linea-sdk-viem';
     *
     * const client = createWalletClient({
     *   chain: mainnet,
     *   transport: http(),
     *   account: privateKeyToAccount('0x…'),
     * }).extend(walletActionsL1());
     *
     * const l2Client = createPublicClient({
     *   chain: linea,
     *   transport: http(),
     * });
     *
     * const hash = await client.deposit({
     *   l2Client,
     *   amount: 1_000_000_000_000n,
     *   token: zeroAddress, // Use zeroAddress for ETH
     *   to: '0xRecipientAddress',
     *   data: '0x', // Optional data
     *   fee: 100_000_000n, // Optional fee
     * });
     */
    deposit: <
      chainOverride extends Chain | undefined = Chain | undefined,
      chainL2 extends Chain | undefined = Chain | undefined,
      accountL2 extends Account | undefined = Account | undefined,
      derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
    >(
      args: DepositParameters<chain, account, chainOverride, chainL2, accountL2, derivedChain>,
    ) => Promise<DepositReturnType>;

    /**
     * Claim a message on L1.
     *
     * @param args - {@link ClaimOnL1Parameters}
     * @returns hash - The [Transaction](https://viem.sh/docs/glossary/terms#transaction) hash. {@link ClaimOnL1ReturnType}
     *
     * @example
     * import { createWalletClient, http } from 'viem';
     * import { privateKeyToAccount } from 'viem/accounts';
     * import { mainnet } from 'viem/chains';
     * import { walletActionsL1 } from '@consensys/linea-sdk-viem';
     *
     * const client = createWalletClient({
     *   chain: mainnet,
     *   transport: http(),
     * }).extend(walletActionsL1());
     *
     * const hash = await client.claimOnL1({
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
     * @example Account Hoisting
     * import { createWalletClient, http } from 'viem';
     * import { privateKeyToAccount } from 'viem/accounts';
     * import { mainnet } from 'viem/chains';
     * import { walletActionsL1 } from '@consensys/linea-sdk-viem';
     *
     * const client = createWalletClient({
     *   account: privateKeyToAccount('0x…'),
     *   chain: mainnet,
     *   transport: http(),
     * }).extend(walletActionsL1());
     *
     * const hash = await client.claimOnL1({
     *     from: '0xSenderAddress',
     *     to: '0xRecipientAddress',
     *     fee: 100_000_000n, // Fee in wei
     *     value: 1_000_000_000_000n, // Amount in wei
     *     messageNonce: 1n, // Nonce of the message to claim
     *     calldata: '0x',
     *     feeRecipient: zeroAddress, // Optional fee recipient, defaults to zeroAddress
     *     messageProof: {
     *        root: '0x…', // Merkle root of the message
     *        proof: ['0x…'], // Merkle proof of the message
     *        leafIndex: 0, // Index of the leaf in the Merkle tree
     *     },
     *     // Optional transaction parameters
     *     gas: 21000n, // Gas limit
     *     maxFeePerGas: 100_000_000n, // Max fee per gas
     *     maxPriorityFeePerGas: 1_000_000n, // Max priority fee per gas
     * });
     */
    claimOnL1: <
      chainOverride extends Chain | undefined = Chain | undefined,
      derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
    >(
      args: ClaimOnL1Parameters<chain, account, chainOverride, derivedChain>,
    ) => Promise<ClaimOnL1ReturnType>;
  }
>;

export type WalletActionsL1Parameters = {
  lineaRollupAddress: Address;
  l2MessageServiceAddress: Address;
  l1TokenBridgeAddress: Address;
  l2TokenBridgeAddress: Address;
};

export function walletActionsL1(parameters?: WalletActionsL1Parameters) {
  return <
    TChain extends Chain | undefined = Chain | undefined,
    TAccount extends Account | undefined = Account | undefined,
  >(
    client: Client<Transport, TChain, TAccount>,
  ): WalletActionsL1 => ({
    deposit: (args) =>
      deposit(client, {
        ...args,
        ...(parameters
          ? {
              lineaRollupAddress: parameters.lineaRollupAddress,
              l2MessageServiceAddress: parameters.l2MessageServiceAddress,
              l1TokenBridgeAddress: parameters.l1TokenBridgeAddress,
              l2TokenBridgeAddress: parameters.l2TokenBridgeAddress,
            }
          : {}),
      }),
    claimOnL1: (args) =>
      claimOnL1(client, { ...args, ...(parameters ? { lineaRollupAddress: parameters.lineaRollupAddress } : {}) }),
  });
}
