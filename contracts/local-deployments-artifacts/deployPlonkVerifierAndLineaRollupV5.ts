import { ethers } from "ethers";
import fs from "fs";
import path from "path";
import * as dotenv from "dotenv";
import { abi as LineaRollupV5Abi, bytecode as LineaRollupV5Bytecode } from "./dynamic-artifacts/LineaRollupV5.json";
import {
  contractName as ProxyAdminContractName,
  abi as ProxyAdminAbi,
  bytecode as ProxyAdminBytecode,
} from "./static-artifacts/ProxyAdmin.json";
import {
  abi as TransparentUpgradeableProxyAbi,
  bytecode as TransparentUpgradeableProxyBytecode,
} from "./static-artifacts/TransparentUpgradeableProxy.json";
import { getRequiredEnvVar } from "../common/helpers/environment";
import { deployContractFromArtifacts, getInitializerData } from "../common/helpers/deployments";
import { get1559Fees } from "../scripts/utils";

dotenv.config();

function findContractArtifacts(
  folderPath: string,
  contractName: string,
): { abi: ethers.InterfaceAbi; bytecode: ethers.BytesLike } {
  const files = fs.readdirSync(folderPath);

  const foundFile = files.find((file) => file === `${contractName}.json`);

  if (!foundFile) {
    // Throw an error if the file is not found
    throw new Error(`Contract "${contractName}" not found in folder "${folderPath}"`);
  }

  // Construct the full file path
  const filePath = path.join(folderPath, foundFile);

  // Read the file content
  const fileContent = fs.readFileSync(filePath, "utf-8").trim();
  const parsedContent = JSON.parse(fileContent);
  return parsedContent;
}

async function main() {
  const verifierName = getRequiredEnvVar("VERIFIER_CONTRACT_NAME");
  const lineaRollupInitialStateRootHash = getRequiredEnvVar("LINEA_ROLLUP_INITIAL_STATE_ROOT_HASH");
  const lineaRollupInitialL2BlockNumber = getRequiredEnvVar("LINEA_ROLLUP_INITIAL_L2_BLOCK_NUMBER");
  const lineaRollupSecurityCouncil = getRequiredEnvVar("LINEA_ROLLUP_SECURITY_COUNCIL");
  const lineaRollupOperators = getRequiredEnvVar("LINEA_ROLLUP_OPERATORS").split(",");
  const lineaRollupRateLimitPeriodInSeconds = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_PERIOD");
  const lineaRollupTateLimitAmountInWei = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_AMOUNT");
  const lineaRollupGenesisTimestamp = getRequiredEnvVar("LINEA_ROLLUP_GENESIS_TIMESTAMP");
  const lineaRollupName = "LineaRollupV5";
  const lineaRollupImplementationName = "LineaRollupV5Implementation";
  const verifierArtifacts = findContractArtifacts(path.join(__dirname, "./dynamic-artifacts"), verifierName);

  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.PRIVATE_KEY!, provider);

  const { gasPrice } = await get1559Fees(provider);

  let walletNonce;

  if (!process.env.L1_NONCE) {
    walletNonce = await wallet.getNonce();
  } else {
    walletNonce = parseInt(process.env.L1_NONCE);
  }

  const [verifier, lineaRollupImplementation, proxyAdmin] = await Promise.all([
    deployContractFromArtifacts(verifierName, verifierArtifacts.abi, verifierArtifacts.bytecode, wallet, {
      nonce: walletNonce,
      gasPrice,
    }),
    deployContractFromArtifacts(lineaRollupImplementationName, LineaRollupV5Abi, LineaRollupV5Bytecode, wallet, {
      nonce: walletNonce + 1,
      gasPrice,
    }),
    deployContractFromArtifacts(ProxyAdminContractName, ProxyAdminAbi, ProxyAdminBytecode, wallet, {
      nonce: walletNonce + 2,
      gasPrice,
    }),
  ]);

  const proxyAdminAddress = await proxyAdmin.getAddress();
  const verifierAddress = await verifier.getAddress();
  const lineaRollupImplementationAddress = await lineaRollupImplementation.getAddress();

  const initializer = getInitializerData(LineaRollupV5Abi, "initialize", [
    lineaRollupInitialStateRootHash,
    lineaRollupInitialL2BlockNumber,
    verifierAddress,
    lineaRollupSecurityCouncil,
    lineaRollupOperators,
    lineaRollupRateLimitPeriodInSeconds,
    lineaRollupTateLimitAmountInWei,
    lineaRollupGenesisTimestamp,
  ]);

  await deployContractFromArtifacts(
    lineaRollupName,
    TransparentUpgradeableProxyAbi,
    TransparentUpgradeableProxyBytecode,
    wallet,
    lineaRollupImplementationAddress,
    proxyAdminAddress,
    initializer,
    { gasPrice },
  );
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
