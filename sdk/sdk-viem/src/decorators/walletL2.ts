import { L2WalletClient } from "@consensys/linea-sdk-core";
import { Account, Address, Chain, Client, DeriveChain, Transport } from "viem";

import { claimOnL2, ClaimOnL2Parameters, ClaimOnL2ReturnType } from "../actions/claimOnL2";
import { withdraw, WithdrawParameters, WithdrawReturnType } from "../actions/withdraw";
import { StrictFunctionOnly } from "../types/misc";

export type WalletActionsL2<
  chain extends Chain | undefined = Chain | undefined,
  account extends Account | undefined = Account | undefined,
> = StrictFunctionOnly<
  L2WalletClient,
  {
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
     * import { walletActionsL2 } from '@consensys/linea-sdk-viem'
     *
     * const client = createWalletClient({
     *   chain: linea,
     *   transport: http(),
     * }).extend(walletActionsL2());
     *
     * const hash = await client.withdraw({
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
     * import { walletActionsL2 } from '@consensys/linea-sdk-viem'
     *
     * const client = createWalletClient({
     *   account: privateKeyToAccount('0x…'),
     *   chain: linea,
     *   transport: http(),
     * }).extend(walletActionsL2());
     *
     * const hash = await client.withdraw({
     *     amount: 1_000_000_000_000n,
     *     token: zeroAddress, // Use zeroAddress for ETH
     *     to: '0xRecipientAddress',
     *     data: '0x', // Optional data
     * });
     */
    withdraw: <
      chainOverride extends Chain | undefined = Chain | undefined,
      derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
    >(
      args: WithdrawParameters<chain, account, chainOverride, derivedChain>,
    ) => Promise<WithdrawReturnType>;

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
     * import { walletActionsL2 } from '@consensys/linea-sdk-viem'
     *
     * const client = createWalletClient({
     *   chain: linea,
     *   transport: http(),
     * }).extend(walletActionsL2());
     *
     * const hash = await client.claimOnL2({
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
     * import { walletActionsL2 } from '@consensys/linea-sdk-viem'
     *
     * const client = createWalletClient({
     *   account: privateKeyToAccount('0x…'),
     *   chain: linea,
     *   transport: http(),
     * });
     *
     * const hash = await client.claimOnL2({
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
    claimOnL2: <
      chainOverride extends Chain | undefined = Chain | undefined,
      derivedChain extends Chain | undefined = DeriveChain<chain, chainOverride>,
    >(
      args: ClaimOnL2Parameters<chain, account, chainOverride, derivedChain>,
    ) => Promise<ClaimOnL2ReturnType>;
  }
>;

type WalletActionsL2Parameters = {
  l2MessageServiceAddress: Address;
  l2TokenBridgeAddress: Address;
};

export function walletActionsL2(parameters?: WalletActionsL2Parameters) {
  return <
    TChain extends Chain | undefined = Chain | undefined,
    TAccount extends Account | undefined = Account | undefined,
  >(
    client: Client<Transport, TChain, TAccount>,
  ): WalletActionsL2 => ({
    withdraw: (args) =>
      withdraw(client, {
        ...args,
        ...(parameters
          ? {
              l2MessageServiceAddress: parameters.l2MessageServiceAddress,
              l2TokenBridgeAddress: parameters.l2TokenBridgeAddress,
            }
          : {}),
      }),
    claimOnL2: (args) =>
      claimOnL2(client, {
        ...args,
        ...(parameters ? { l2MessageServiceAddress: parameters.l2MessageServiceAddress } : {}),
      }),
  });
}
