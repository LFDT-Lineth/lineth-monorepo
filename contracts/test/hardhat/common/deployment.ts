import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import hre from "hardhat";
import { type Abi, type Address, type Hash, encodeFunctionData, getAddress, parseAbi } from "viem";

import { getConnection } from "./helpers/connection.js";

import type {
  ConstructorArgs,
  ContractReturnType,
  DeployContractConfig,
  KeyedClient,
} from "@nomicfoundation/hardhat-viem/types";
import type { DeployProxyOptions, HardhatViemUpgrades, Libraries } from "@openzeppelin/hardhat-upgrades/viem";
import type { NetworkConnection } from "hardhat/types/network";

/** OpenZeppelin `ProxyAdmin.upgradeAndCall` (same signature in Contracts 4.x and 5.x). */
const PROXY_ADMIN_UPGRADE_AND_CALL_ABI = parseAbi([
  "function upgradeAndCall(address proxy, address implementation, bytes data) payable",
]);

const upgradesByConnection = new WeakMap<NetworkConnection, Promise<HardhatViemUpgrades>>();

/** OpenZeppelin Upgrades (viem API) bound to the test connection (see `getConnection`); created once per connection. */
export async function getUpgrades(): Promise<HardhatViemUpgrades> {
  const connection = await getConnection();
  let api = upgradesByConnection.get(connection);
  if (api === undefined) {
    api = upgrades(hre, connection);
    upgradesByConnection.set(connection, api);
  }
  return api;
}

async function deployFromFactory<ContractName extends string>(
  contractName: ContractName,
  constructorArgs?: ConstructorArgs<ContractName>,
  config?: DeployContractConfig,
): Promise<ContractReturnType<ContractName>> {
  const { viem } = await getConnection();
  const contract = await viem.deployContract(contractName, constructorArgs, config);
  return withChecksummedAddress(contractName, contract, config?.client);
}

/**
 * Re-binds a freshly deployed contract instance to its checksummed address (deployment receipts report it in
 * lowercase; ethers returned it checksummed), keeping the given clients.
 */
async function withChecksummedAddress<ContractName extends string>(
  contractName: ContractName,
  contract: ContractReturnType<ContractName>,
  client?: KeyedClient,
): Promise<ContractReturnType<ContractName>> {
  const checksummed = getAddress(contract.address);
  if (checksummed === contract.address) {
    return contract;
  }
  const { viem } = await getConnection();
  return client === undefined
    ? viem.getContractAt(contractName, checksummed)
    : viem.getContractAt(contractName, checksummed, { client });
}

async function deployUpgradableFromFactory<ContractName extends string>(
  contractName: ContractName,
  args?: unknown[],
  opts?: DeployProxyOptions,
  factoryOpts?: { libraries?: Libraries },
): Promise<ContractReturnType<ContractName>> {
  const proxy = await (await getUpgrades()).deployProxy(contractName, args, { ...opts, ...factoryOpts });
  return withChecksummedAddress(contractName, proxy, opts?.client);
}

// Use constructor in upgradeable contract to set immutable for static global constants.
async function deployUpgradableWithConstructorArgs<ContractName extends string>(
  contractName: ContractName,
  constructorArgs: unknown[] = [],
  initializerArgs: unknown[] = [],
  opts: DeployProxyOptions = {},
  factoryOpts?: { libraries?: Libraries },
): Promise<ContractReturnType<ContractName>> {
  const proxy = await (
    await getUpgrades()
  ).deployProxy(contractName, initializerArgs, {
    ...opts,
    ...factoryOpts,
    constructorArgs,
  });
  return withChecksummedAddress(contractName, proxy, opts.client);
}

/**
 * Reinitializes an upgradeable proxy by calling `upgradeAndCall` on its ProxyAdmin (read from the ERC-1967 admin
 * slot), re-pointing it at its current implementation with the encoded reinitializer call.
 *
 * @param proxyContract - The proxy contract instance to reinitialize
 * @param contractAbi - The ABI of the contract (used to encode the initializer call)
 * @param initializerFunctionName - The name of the reinitializer function to call
 * @param initializerArgs - The arguments to pass to the reinitializer function
 * @returns The transaction hash of the `upgradeAndCall` call, sent by the first test account (the admin owner)
 */
async function reinitializeUpgradeableProxy(
  proxyContract: { address: Address },
  contractAbi: Abi,
  initializerFunctionName: string,
  initializerArgs: readonly unknown[],
): Promise<Hash> {
  const { viem } = await getConnection();
  const oz = await getUpgrades();
  const proxyAdmin = await oz.erc1967.getAdminAddress(proxyContract.address);
  const implementation = await oz.erc1967.getImplementationAddress(proxyContract.address);
  const data = encodeFunctionData({ abi: contractAbi, functionName: initializerFunctionName, args: initializerArgs });
  const [owner] = await viem.getWalletClients();
  return owner.writeContract({
    address: proxyAdmin,
    abi: PROXY_ADMIN_UPGRADE_AND_CALL_ABI,
    functionName: "upgradeAndCall",
    args: [proxyContract.address, implementation, data],
  });
}

export {
  deployFromFactory,
  deployUpgradableFromFactory,
  deployUpgradableWithConstructorArgs,
  reinitializeUpgradeableProxy,
};
