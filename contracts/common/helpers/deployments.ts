import { ethers, AbstractSigner, Interface, InterfaceAbi, BaseContract } from "ethers";

import { clearSignerUiWorkflowStatus, setSignerUiWorkflowStatus } from "./signerUiWorkflowStatus";
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

/**
 * Options for deploying a contract with library linking support.
 */
export interface DeployContractOptions {
  /**
   * Libraries to link. Keys should match the placeholder names in the bytecode.
   * Format: { "contracts/path/Library.sol:LibraryName": "0x..." }
   * or simply { "LibraryName": "0x..." } if using short names.
   */
  libraries?: Record<string, string>;
}

function isDeployContractOptions(value: unknown): value is DeployContractOptions {
  return typeof value === "object" && value !== null && "libraries" in value;
}

type DeployArgs<A extends Array<unknown>> = ethers.ContractMethodArgs<A>;
type DeployParams<A extends Array<unknown>> = DeployArgs<A> | [DeployContractOptions, ...DeployArgs<A>];

/**
 * Links library addresses into bytecode by replacing placeholder patterns.
 * Supports both fully qualified names (contracts/path/Lib.sol:Lib) and short names (Lib).
 */
function linkLibraries(bytecode: ethers.BytesLike, libraries: Record<string, string>): string {
  let linked = typeof bytecode === "string" ? bytecode : ethers.hexlify(bytecode);

  for (const [name, address] of Object.entries(libraries)) {
    if (!ethers.isAddress(address)) {
      throw new Error(`Invalid library address for "${name}": ${address}`);
    }
    // Compute placeholder hash: first 34 chars of keccak256(name)
    const hash = ethers.keccak256(ethers.toUtf8Bytes(name)).slice(2, 36);
    const placeholder = new RegExp(`__\\$${hash}\\$__`, "g");
    const addressWithoutPrefix = ethers.getAddress(address).slice(2).toLowerCase();
    linked = linked.replace(placeholder, addressWithoutPrefix);
  }

  // Verify no unlinked placeholders remain
  const unlinkedMatch = linked.match(/__\$[a-fA-F0-9]{34}\$__/);
  if (unlinkedMatch) {
    throw new Error(`Bytecode contains unlinked library placeholder: ${unlinkedMatch[0]}`);
  }

  return linked;
}

/**
 * Deploys a contract from artifact data.
 *
 * @param contractName - Name for logging purposes
 * @param abi - Contract ABI
 * @param bytecode - Contract bytecode (may contain library placeholders)
 * @param wallet - Signer or contract runner for deployment
 * @param args - Constructor arguments, optionally preceded by DeployContractOptions
 *
 * @example
 * // Without libraries (existing usage - unchanged)
 * await deployContractFromArtifacts("MyContract", abi, bytecode, wallet, arg1, arg2);
 *
 * @example
 * // With libraries
 * await deployContractFromArtifacts("MyContract", abi, bytecode, wallet,
 *   { libraries: { "contracts/libraries/Mimc.sol:Mimc": mimcAddress } },
 *   arg1, arg2
 * );
 */
export async function deployContractFromArtifacts<A extends Array<unknown>>(
  contractName: string,
  abi: ethers.InterfaceAbi,
  bytecode: ethers.BytesLike,
  wallet: AbstractSigner | ethers.ContractRunner,
  ...args: DeployParams<A>
): Promise<BaseContract> {
  let options: DeployContractOptions = {};
  let constructorArgs = args as DeployArgs<A>;

  const [firstArg, ...restArgs] = args;
  // Check if first arg is options object (has 'libraries' property)
  if (isDeployContractOptions(firstArg)) {
    options = firstArg;
    constructorArgs = restArgs as DeployArgs<A>;
  }

  // Link libraries if provided
  const linkedBytecode = options.libraries ? linkLibraries(bytecode, options.libraries) : bytecode;

  const factory = new ethers.ContractFactory(abi, linkedBytecode, wallet);
  const contract = await factory.deploy(...constructorArgs);

  await LogContractDeployment(contractName, contract);

  return contract;
}

export async function LogContractDeployment(contractName: string, contract: BaseContract) {
  const deploymentTx = contract.deploymentTransaction();
  if (!deploymentTx) {
    throw new Error("Deployment transaction not found.");
  }

  const receiptPending = deploymentTx.blockNumber === null || deploymentTx.blockNumber === undefined;
  if (receiptPending) {
    await setSignerUiWorkflowStatus(
      "waiting_for_transaction_receipt",
      `Waiting for transaction receipt for ${contractName}.`,
    );
  }

  let txReceipt;
  try {
    txReceipt = await deploymentTx.wait();
  } finally {
    if (receiptPending) {
      await clearSignerUiWorkflowStatus();
    }
  }

  if (!txReceipt) {
    throw new Error("Deployment transaction not found.");
  }

  const contractAddress = await contract.getAddress();
  const chainId = (await deploymentTx.provider.getNetwork()).chainId;
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
