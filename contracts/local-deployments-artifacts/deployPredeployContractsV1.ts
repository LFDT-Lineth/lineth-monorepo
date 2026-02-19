import { ethers } from "ethers";
import * as dotenv from "dotenv";
import {
  contractName as ConsolidationQueueContractName,
  abi as ConsolidationQueueAbi,
  bytecode as ConsolidationQueueBytecode,
} from "./dynamic-artifacts/UpgradeableConsolidationQueuePredeployV1.json";
import {
  contractName as BeaconChainDepositContractName,
  abi as BeaconChainDepositAbi,
  bytecode as BeaconChainDepositBytecode,
} from "./dynamic-artifacts/UpgradeableBeaconChainDepositPredeployV1.json";
import {
  contractName as WithdrawalQueueContractName,
  abi as WithdrawalQueueAbi,
  bytecode as WithdrawalQueueBytecode,
} from "./dynamic-artifacts/UpgradeableWithdrawalQueuePredeployV1.json";
import {
  contractName as ProxyAdminContractName,
  abi as ProxyAdminAbi,
  bytecode as ProxyAdminBytecode,
} from "./static-artifacts/ProxyAdmin.json";
import {
  abi as TransparentUpgradeableProxyAbi,
  bytecode as TransparentUpgradeableProxyBytecode,
} from "./static-artifacts/TransparentUpgradeableProxy.json";
import { deployContractFromArtifacts, getInitializerData } from "../common/helpers/deployments";

dotenv.config();

async function main() {
  console.log("Starting deployment of predeploy contracts...");

  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);
  let walletNonce;

  if (!process.env.L2_NONCE) {
    walletNonce = await wallet.getNonce();
  } else {
    walletNonce = parseInt(process.env.L2_NONCE);
  }
  console.log("walletNonce:", walletNonce);

  // Contract implementation names
  const consolidationQueueImplName = "UpgradeableConsolidationQueuePredeployImplementation";
  const beaconChainDepositImplName = "UpgradeableBeaconChainDepositPredeployImplementation";
  const withdrawalQueueImplName = "UpgradeableWithdrawalQueuePredeployImplementation";

  // Deploy all implementation contracts and ProxyAdmin in parallel
  console.log(
    `Deploying ${consolidationQueueImplName}, ${beaconChainDepositImplName}, ${withdrawalQueueImplName} and ProxyAdmin...`,
  );
  const [
    consolidationQueueImplementation,
    beaconChainDepositImplementation,
    withdrawalQueueImplementation,
    proxyAdmin,
  ] = await Promise.all([
    deployContractFromArtifacts(consolidationQueueImplName, ConsolidationQueueAbi, ConsolidationQueueBytecode, wallet, {
      nonce: walletNonce,
    }),
    deployContractFromArtifacts(beaconChainDepositImplName, BeaconChainDepositAbi, BeaconChainDepositBytecode, wallet, {
      nonce: walletNonce + 1,
    }),
    deployContractFromArtifacts(withdrawalQueueImplName, WithdrawalQueueAbi, WithdrawalQueueBytecode, wallet, {
      nonce: walletNonce + 2,
    }),
    deployContractFromArtifacts(ProxyAdminContractName, ProxyAdminAbi, ProxyAdminBytecode, wallet, {
      nonce: walletNonce + 3,
    }),
  ]);

  const proxyAdminAddress = await proxyAdmin.getAddress();
  const consolidationQueueImplAddress = await consolidationQueueImplementation.getAddress();
  const beaconChainDepositImplAddress = await beaconChainDepositImplementation.getAddress();
  const withdrawalQueueImplAddress = await withdrawalQueueImplementation.getAddress();

  // Get initializer data for empty initialize() function
  const emptyInitializer = getInitializerData(ConsolidationQueueAbi, "initialize", []);

  // Deploy proxy contracts in parallel
  const [consolidationQueueProxy, beaconChainDepositProxy, withdrawalQueueProxy] = await Promise.all([
    deployContractFromArtifacts(
      ConsolidationQueueContractName,
      TransparentUpgradeableProxyAbi,
      TransparentUpgradeableProxyBytecode,
      wallet,
      consolidationQueueImplAddress,
      proxyAdminAddress,
      emptyInitializer,
      { nonce: walletNonce + 4 },
    ),
    deployContractFromArtifacts(
      BeaconChainDepositContractName,
      TransparentUpgradeableProxyAbi,
      TransparentUpgradeableProxyBytecode,
      wallet,
      beaconChainDepositImplAddress,
      proxyAdminAddress,
      emptyInitializer,
      { nonce: walletNonce + 5 },
    ),
    deployContractFromArtifacts(
      WithdrawalQueueContractName,
      TransparentUpgradeableProxyAbi,
      TransparentUpgradeableProxyBytecode,
      wallet,
      withdrawalQueueImplAddress,
      proxyAdminAddress,
      emptyInitializer,
      { nonce: walletNonce + 6 },
    ),
  ]);

  const consolidationQueueProxyAddress = await consolidationQueueProxy.getAddress();
  const beaconChainDepositProxyAddress = await beaconChainDepositProxy.getAddress();
  const withdrawalQueueProxyAddress = await withdrawalQueueProxy.getAddress();

  console.log("\n=== DEPLOYMENT COMPLETE ===");
  console.log(`ProxyAdmin: ${proxyAdminAddress}`);
  console.log(`UpgradeableConsolidationQueuePredeploy proxy: ${consolidationQueueProxyAddress}`);
  console.log(`UpgradeableBeaconChainDepositPredeploy proxy: ${beaconChainDepositProxyAddress}`);
  console.log(`UpgradeableWithdrawalQueuePredeploy proxy: ${withdrawalQueueProxyAddress}`);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
