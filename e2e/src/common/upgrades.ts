import { AbstractSigner, ContractTransactionReceipt, ethers } from "ethers";
import { ProxyAdmin, ProxyAdmin__factory } from "../typechain";

export function getInitializerData(
  contractInterface: ethers.Interface,
  initializerFunctionName: string,
  args: unknown[],
) {
  const fragment = contractInterface.getFunction(initializerFunctionName);
  return contractInterface.encodeFunctionData(fragment!, args);
}

export async function upgradeContractAndCall(
  deployer: AbstractSigner,
  proxyAdminContractAddress: string,
  proxyContractAddress: string,
  implementationContractAddress: string,
  initializerData = "0x",
): Promise<ContractTransactionReceipt | null> {
  const proxyAdminFactory = new ProxyAdmin__factory(deployer);
  const proxyAdmin = proxyAdminFactory.connect(deployer).attach(proxyAdminContractAddress) as ProxyAdmin;
  const tx = await proxyAdmin.upgradeAndCall(proxyContractAddress, implementationContractAddress, initializerData);
  return tx.wait();
}

export async function upgradeContract(
  deployer: AbstractSigner,
  proxyAdminContractAddress: string,
  proxyContractAddress: string,
  implementationContractAddress: string,
): Promise<ContractTransactionReceipt | null> {
  const proxyAdminFactory = new ProxyAdmin__factory(deployer);
  const proxyAdmin = proxyAdminFactory.connect(deployer).attach(proxyAdminContractAddress) as ProxyAdmin;
  const tx = await proxyAdmin.upgrade(proxyContractAddress, implementationContractAddress);

  return tx.wait();
}
