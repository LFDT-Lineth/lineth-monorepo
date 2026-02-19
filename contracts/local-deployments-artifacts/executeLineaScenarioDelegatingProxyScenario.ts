/*
    *******************************************************************************************
    1. Set the RPC_URL 
    2. Set the DEPLOYER_PRIVATE_KEY
    3. Set LINEA_SCENARIO_DELEGATING_PROXY_ADDRESS
    4. Set NUMBER_OF_LOOPS
    5. set LINEA_SCENARIO
    6. set GAS_LIMIT
    *******************************************************************************************
    *******************************************************************************************
    LINEA_SCENARIO_DELEGATING_PROXY_ADDRESS=<address> \
    NUMBER_OF_LOOPS=<number> \
    LINEA_SCENARIO=<number> \
    GAS_LIMIT=<number> \
    DEPLOYER_PRIVATE_KEY=<key> \
    RPC_URL=<url> \
    npx ts-node local-deployments-artifacts/executeLineaScenarioDelegatingProxyScenario.ts
    *******************************************************************************************
*/

import { getRequiredEnvVar } from "../common/helpers/environment";
import { ethers } from "ethers";
import { abi as testerAbi } from "./static-artifacts/LineaScenarioDelegatingProxy.json";

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);

  const testContractAddress = getRequiredEnvVar("LINEA_SCENARIO_DELEGATING_PROXY_ADDRESS");
  const lineaScenario = 1; //getRequiredEnvVar("LINEA_SCENARIO");
  const numberOfLoops = getRequiredEnvVar("NUMBER_OF_LOOPS");
  const gasLimit = getRequiredEnvVar("GAS_LIMIT");

  // Equivalent of getContractAt
  const delegatingProxy = new ethers.Contract(testContractAddress, testerAbi, wallet);
  const executeTx = await delegatingProxy.executeScenario(lineaScenario, numberOfLoops, { gasLimit: gasLimit });
  try {
    const receipt = await executeTx.wait();
    console.log(`Executed transaction with gasUsed=${receipt?.gasUsed} status=${receipt?.status}`);
  } catch {
    const receipt = await provider.getTransactionReceipt(executeTx.hash);
    console.error("Transaction failed - tx receipt=", JSON.stringify(receipt));
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
