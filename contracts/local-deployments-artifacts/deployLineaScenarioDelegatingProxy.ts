import { ethers } from "ethers";

import lineaScenarioDelegatingProxyJson from "./static-artifacts/LineaScenarioDelegatingProxy.json" with { type: "json" };
import { deployContractFromArtifacts } from "../common/helpers/deployments.js";

const {
  contractName: lineaScenarioDelegatingProxyName,
  abi: lineaScenarioDelegatingProxyAbi,
  bytecode: lineaScenarioDelegatingProxyBytecode,
} = lineaScenarioDelegatingProxyJson;

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);

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
