import { ethers } from "ethers";
import {
  contractName as lineaScenarioDelegatingProxyName,
  abi as lineaScenarioDelegatingProxyAbi,
  bytecode as lineaScenarioDelegatingProxyBytecode,
} from "./static-artifacts/LineaScenarioDelegatingProxy.json";
import { deployContractFromArtifacts } from "../common/helpers/deployments";

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.PRIVATE_KEY!, provider);

  console.log(`Deploying LineaScenarioDelegatingProxy`);
  const lineaScenarioDelegatingProxyAddress = await deploylineaScenarioDelegatingProxy(wallet);
  console.log(
    `LineaScenarioDelegatingProxy Deployed at lineaScenarioDelegatingProxyAddress=${lineaScenarioDelegatingProxyAddress}`,
  );
}

async function deploylineaScenarioDelegatingProxy(wallet: ethers.Wallet): Promise<string> {
  const walletNonce = await wallet.getNonce();

  const lineaScenarioDelegatingProxy = await deployContractFromArtifacts(
    lineaScenarioDelegatingProxyName,
    lineaScenarioDelegatingProxyAbi,
    lineaScenarioDelegatingProxyBytecode,
    wallet,
    {
      nonce: walletNonce,
    },
  );

  const lineaScenarioDelegatingProxyAddress = await lineaScenarioDelegatingProxy.getAddress();

  return lineaScenarioDelegatingProxyAddress;
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
