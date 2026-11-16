import { ethers, type AbstractSigner, Interface, type InterfaceAbi, type BaseContract } from "ethers";
import fs from "fs";
import path from "path";

import { normalizeAddressArgs } from "./normalize-address-args.js";

/**
 * Loads a compiled contract artifact ("<contractName>.json") from a directory.
 * Replaces the per-script `findContractArtifacts` copies in local-deployments-artifacts.
 */
export function loadArtifactFromDirectory(
  folderPath: string,
  contractName: string,
): { abi: ethers.InterfaceAbi; bytecode: ethers.BytesLike } {
  const fileName = `${contractName}.json`;
  const filePath = path.join(folderPath, fileName);
  if (!fs.existsSync(filePath)) {
    throw new Error(`Contract "${contractName}" not found in folder "${folderPath}"`);
  }
  const fileContent = fs.readFileSync(filePath, "utf-8").trim();
  return JSON.parse(fileContent);
}

/**
 * Resolves a deploy nonce from an env var (e.g. "L1_NONCE"/"L2_NONCE"), validated as a
 * non-negative integer. When the var is unset/empty, falls back to the wallet's live nonce.
 * `offset` is added only when the env var is set, for scripts that deploy in a fixed order from a base nonce.
 *
 * Replaces the unvalidated `parseInt(process.env.L1_NONCE)` pattern across deploy scripts.
 */
export async function getDeployNonceFromEnv(
  wallet: Pick<AbstractSigner, "getNonce">,
  envVarName: string,
  offset = 0,
): Promise<number> {
  const raw = process.env[envVarName];
  if (raw === undefined || raw === "") {
    return await wallet.getNonce();
  }
  if (!/^[0-9]+$/.test(raw)) {
    throw new Error(`${envVarName} must be a non-negative integer, got: ${raw}`);
  }
  return Number(raw) + offset;
}

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
  const normalizedArgs = await normalizeAddressArgs(factory, constructorArgs as unknown[]);
  const contract = await factory.deploy(...normalizedArgs);

  await LogContractDeployment(contractName, contract);

  return contract;
}

export async function LogContractDeployment(contractName: string, contract: BaseContract) {
  const deploymentTx = contract.deploymentTransaction();
  if (!deploymentTx) {
    throw new Error("Deployment transaction not found.");
  }

  const txReceipt = await deploymentTx.wait();
  if (!txReceipt) {
    throw new Error("Deployment transaction not found.");
  }

  const contractAddress = await contract.getAddress();
  const chainId = (await deploymentTx.provider.getNetwork()).chainId;
  console.log(
    `contract=${contractName} deployed: address=${contractAddress} blockNumber=${txReceipt.blockNumber} chainId=${chainId}`,
  );
}
