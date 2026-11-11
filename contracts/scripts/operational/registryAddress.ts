import { type Address, getAddress } from "viem";

import { getAddressFromRegistry } from "../../common/helpers/readAddress.js";

/**
 * Registry keys (`deployments/addresses/<network>.json`) for the contract names the operational tasks used to
 * resolve through hardhat-deploy's `deployments.get(<name>)`. Lookup tries the contract name first, then this key.
 */
const REGISTRY_KEY_BY_CONTRACT_NAME: Record<string, string> = {
  LinethRollup: "LINETH_ROLLUP_ADDRESS",
  L2MessageService: "L2_MESSAGE_SERVICE_ADDRESS",
  TokenBridge: "TOKEN_BRIDGE_ADDRESS",
  BridgedToken: "BRIDGED_TOKEN_ADDRESS",
  YieldManager: "YIELD_MANAGER_ADDRESS",
  RollupRevenueVault: "ROLLUP_REVENUE_VAULT_ADDRESS",
};

/**
 * Replaces the Hardhat 2 `deployments.get(contractName).address` fallback with the address registry.
 * Throws when the network has no registry entry for the contract.
 */
export function requireRegistryAddress(networkName: string, contractName: string): Address {
  const registryKey = REGISTRY_KEY_BY_CONTRACT_NAME[contractName];
  const address = getAddressFromRegistry(networkName, contractName, registryKey);
  if (address === undefined) {
    throw new Error(
      `No address found for "${contractName}" on network "${networkName}" in deployments/addresses/${networkName}.json` +
        `${registryKey ? ` (key ${contractName} or ${registryKey})` : ""}. Pass the address explicitly.`,
    );
  }
  return getAddress(address);
}
