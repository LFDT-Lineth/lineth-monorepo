import { ethers, AbstractSigner, Interface, InterfaceAbi, BaseContract } from "ethers";
import {
  contractName as ProxyAdminContractName,
  abi as ProxyAdminAbi,
  bytecode as ProxyAdminBytecode,
} from "../../deployments/bytecode/mainnet-proxy/ProxyAdmin.json";
import {
  contractName as TransparentUpgradeableProxyContractName,
  abi as TransparentUpgradeableProxyAbi,
  bytecode as TransparentUpgradeableProxyBytecode,
} from "../../deployments/bytecode/mainnet-proxy/TransparentUpgradeableProxy.json";

export function getInitializerData(contractAbi: InterfaceAbi, initializerFunctionName: string, args: unknown[]) {
  const contractInterface = new Interface(contractAbi);
  const fragment = contractInterface.getFunction(initializerFunctionName);

  if (!fragment) {
    return "0x";
  }

  return contractInterface.encodeFunctionData(fragment, args);
}

export async function deployContractFromArtifacts(
  contractName: string,
  abi: ethers.InterfaceAbi,
  bytecode: ethers.BytesLike,
  wallet: AbstractSigner | ethers.ContractRunner,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ...args: ethers.ContractMethodArgs<any[]>
) {
  const factory = new ethers.ContractFactory(abi, bytecode, wallet);
  const contract = await factory.deploy(...args);

  await LogContractDeployment(contractName, contract);

  return contract;
}

export async function LogContractDeployment(contractName: string, contract: BaseContract) {
  const txReceipt = await contract.deploymentTransaction()?.wait();
  if (!txReceipt) {
    throw "Deployment transaction not found.";
  }

  const contractAddress = await contract.getAddress();
  const chainId = (await contract.deploymentTransaction()!.provider.getNetwork()).chainId;
  console.log(
    `contract=${contractName} deployed: address=${contractAddress} blockNumber=${txReceipt.blockNumber} chainId=${chainId}`,
  );
}

/**
 * Deploys a ProxyAdmin (if needed) and a TransparentUpgradeableProxy for a given implementation contract.
 *
 * This function handles the deployment of OpenZeppelin V4 upgradeable proxy infrastructure:
 * - Optionally deploys a new ProxyAdmin contract (if not provided)
 * - Deploys a TransparentUpgradeableProxy pointing to the implementation
 *
 * @param implementationAddress - The address of the implementation contract that the proxy will delegate to
 * @param deployer - The signer/contract runner that will deploy the contracts
 * @param initializerData - The encoded initializer function call data (use "0x" for no initialization)
 * @param proxyAdminAddress - **Optional.** If provided, reuses the existing ProxyAdmin at this address.
 *                           If not provided (undefined), deploys a new ProxyAdmin contract.
 *                           This allows you to either:
 *                           - Deploy a new ProxyAdmin for this proxy (when undefined)
 *                           - Reuse an existing ProxyAdmin to manage multiple proxies (when provided)
 *
 * @returns A promise that resolves to an object containing:
 *   - `proxyAddress`: The address of the deployed TransparentUpgradeableProxy
 *   - `proxyAdminAddress`: The address of the ProxyAdmin (either reused or newly deployed)
 */
export async function deployProxyAdminAndProxy(
  implementationAddress: string,
  deployer: ethers.ContractRunner,
  initializerData: string,
  proxyAdminAddress?: string,
): Promise<{ proxyAddress: string; proxyAdminAddress: string }> {
  // Validate implementation address
  if (!ethers.isAddress(implementationAddress)) {
    throw new Error(`Invalid implementation address: ${implementationAddress}`);
  }
  const normalizedImplementationAddress = ethers.getAddress(implementationAddress);

  // Validate proxyAdminAddress if provided
  if (proxyAdminAddress !== undefined && !ethers.isAddress(proxyAdminAddress)) {
    throw new Error(`Invalid proxyAdmin address: ${proxyAdminAddress}`);
  }
  const normalizedProxyAdminAddress = proxyAdminAddress ? ethers.getAddress(proxyAdminAddress) : undefined;

  let finalProxyAdminAddress: string;

  // Deploy ProxyAdmin if not provided
  if (!normalizedProxyAdminAddress) {
    console.log("Deploying new ProxyAdmin contract...");
    const proxyAdmin = await deployContractFromArtifacts(
      ProxyAdminContractName,
      ProxyAdminAbi,
      ProxyAdminBytecode,
      deployer,
    );
    finalProxyAdminAddress = await proxyAdmin.getAddress();
    console.log(`ProxyAdmin deployed at: ${finalProxyAdminAddress}`);
  } else {
    console.log(`Reusing existing ProxyAdmin at: ${normalizedProxyAdminAddress}`);
    finalProxyAdminAddress = normalizedProxyAdminAddress;
  }

  // Deploy TransparentUpgradeableProxy
  const proxy = await deployContractFromArtifacts(
    TransparentUpgradeableProxyContractName,
    TransparentUpgradeableProxyAbi,
    TransparentUpgradeableProxyBytecode,
    deployer,
    normalizedImplementationAddress,
    finalProxyAdminAddress,
    initializerData,
  );

  const proxyAddress = await proxy.getAddress();

  return {
    proxyAddress,
    proxyAdminAddress: finalProxyAdminAddress,
  };
}
