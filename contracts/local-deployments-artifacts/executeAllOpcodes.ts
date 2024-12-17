/*
    *******************************************************************************************
    1. Set the RPC_URL 
    2. Set the PRIVATE_KEY
    3. Set OPCODE_TEST_CONTRACT_ADDRESS
    4. Set NUMBER_OF_RUNS
    *******************************************************************************************
    *******************************************************************************************
    OPCODE_TEST_CONTRACT_ADDRESS=<address> \
    NUMBER_OF_RUNS=<number> \
    PRIVATE_KEY=<key> \
    RPC_URL=<url> \
    npx ts-node local-deployments-artifacts/executeAllOpcodes.ts
    *******************************************************************************************
*/

import { getRequiredEnvVar } from "../common/helpers/environment";
import { ethers } from "ethers";
import { abi as opcodeTesterAbi } from "./static-artifacts/OpcodeTester.json";

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.PRIVATE_KEY!, provider);

  const opcodeTestContractAddress = getRequiredEnvVar("OPCODE_TEST_CONTRACT_ADDRESS");
  const numberOfRuns = getRequiredEnvVar("NUMBER_OF_RUNS");
  const executionRunCount = parseInt(numberOfRuns);

  // Equivalent of getContractAt
  const opcodeTester = new ethers.Contract(opcodeTestContractAddress, opcodeTesterAbi, wallet);

  for (let i = 1; i <= executionRunCount; i++) {
    console.log(`Executing all opcodes for runs ${i} of ${executionRunCount}`);
    const executeTx = await opcodeTester.executeAllOpcodes({ gasLimit: 5_000_000 });
    const receipt = await executeTx.wait();
    console.log(` - Gas used in run: ${receipt?.gasUsed}`);
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
