import { type HDAccount, mnemonicToAccount } from "viem/accounts";

import { getConnection } from "../../common/helpers/connection.js";

/**
 * Derives the local (private-key) account at `m/44'/60'/0'/0/<index>` from the default network's HD mnemonic, so
 * tests can sign raw transactions (e.g. EIP-4844 blob transactions) themselves.
 */
export const getWalletForIndex = async (index: number): Promise<HDAccount> => {
  const { networkConfig } = await getConnection();
  if (networkConfig.type !== "edr-simulated" || Array.isArray(networkConfig.accounts)) {
    throw new Error("getWalletForIndex requires the simulated network configured with HD accounts");
  }
  const mnemonic = await networkConfig.accounts.mnemonic.get();
  return mnemonicToAccount(mnemonic, { addressIndex: index });
};
