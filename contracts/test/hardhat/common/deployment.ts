import { DeployProxyOptions } from "@openzeppelin/hardhat-upgrades/src/utils";
import { getInitializerData } from "contracts/common/helpers";
import { ProxyAdmin } from "contracts/typechain-types";
import { BaseContract, ContractTransactionResponse, InterfaceAbi } from "ethers";
import { ethers, upgrades } from "hardhat";
import { FactoryOptions } from "hardhat/types";

async function deployFromFactory(contractName: string, ...args: unknown[]) {
  const factory = await ethers.getContractFactory(contractName);
  const contract = await factory.deploy(...args);
  await contract.waitForDeployment();
  return contract;
}

async function deployUpgradableFromFactory(
  contractName: string,
  args?: unknown[],
  opts?: DeployProxyOptions,
  factoryOpts?: FactoryOptions,
) {
  const factory = await ethers.getContractFactory(contractName, factoryOpts);
  const contract = await upgrades.deployProxy(factory, args, opts);
  await contract.waitForDeployment();
  return contract;
}

// Use constructor in upgradeable contract to set immutable for static global constants.
async function deployUpgradableWithConstructorArgs(
  contractName: string,
  constructorArgs: unknown[] = [],
  initializerArgs: unknown[] = [],
  opts: DeployProxyOptions = {},
  factoryOpts?: FactoryOptions,
) {
  const factory = await ethers.getContractFactory(contractName, factoryOpts);

  const contract = await upgrades.deployProxy(factory, initializerArgs, {
    ...opts,
    constructorArgs,
  });

  await contract.waitForDeployment();
  return contract;
}

/**
 * Reinitializes an upgradeable proxy contract by calling upgradeAndCall on the ProxyAdmin.
 *
 * @param proxyContract - The proxy contract instance to reinitialize
 * @param contractAbi - The ABI of the contract (used to encode the initializer call)
 * @param initializerFunctionName - The name of the reinitializer function to call
 * @param initializerArgs - The arguments to pass to the reinitializer function
 */
async function reinitializeUpgradeableProxy(
  proxyContract: BaseContract,
  contractAbi: InterfaceAbi,
  initializerFunctionName: string,
  initializerArgs: unknown[],
): Promise<ContractTransactionResponse> {
  const proxyAdmin = (await upgrades.admin.getInstance()) as unknown as ProxyAdmin;
  const proxyAddress = await proxyContract.getAddress();
  const implementationAddress = await upgrades.erc1967.getImplementationAddress(proxyAddress);

  const encodedCall = getInitializerData(contractAbi, initializerFunctionName, initializerArgs);

  return await proxyAdmin.upgradeAndCall(proxyAddress, implementationAddress, encodedCall);
}

export {
  deployFromFactory,
  deployUpgradableFromFactory,
  deployUpgradableWithConstructorArgs,
  reinitializeUpgradeableProxy,
};
