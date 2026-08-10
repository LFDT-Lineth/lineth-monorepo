import { getContractsAddressesByChainId, isMainnet, isSepolia } from "@lfdt-lineth/sdk-core";
import { Address } from "viem";

import { RollupAddressRequiredError, RollupAddressRequiredErrorType } from "../errors/bridge";

export type ResolveRollupAddressErrorType = RollupAddressRequiredErrorType;

/**
 * Resolves the rollup (settlement) contract address for a settlement-chain client.
 *
 * Defaulting only makes sense when the settlement chain is Ethereum Mainnet or Sepolia, where
 * `getContractsAddressesByChainId` resolves to the well-known Linea rollup address. For any other
 * settlement chain (e.g. a Validium chain settling on Linea instead of Ethereum), there is no
 * sensible default — the deployed rollup contract there is specific to that chain and isn't one of
 * the addresses `getContractsAddressesByChainId` knows about — so `rollupAddress` must be provided
 * explicitly.
 */
export function resolveRollupAddress(chainId: number, rollupAddress: Address | undefined): Address {
  if (rollupAddress) {
    return rollupAddress;
  }

  if (!isMainnet(chainId) && !isSepolia(chainId)) {
    throw new RollupAddressRequiredError({ chainId });
  }

  return getContractsAddressesByChainId(chainId).messageService;
}
