// The current supported EVM version is Osaka, which is a hardfork of the EVM.
import { ethers } from "ethers";

import opcodeTesterJson from "./static-artifacts/OpcodeTester.json" with { type: "json" };
import yulBasedOpcodeTestingJson from "./static-artifacts/YulBasedOpcodeTesting.json" with { type: "json" };
import { deployContractFromArtifacts } from "../common/helpers/deployments.js";
import { LOCAL_L2_DEPLOY_FEE_OVERRIDES } from "../common/helpers/feeOverrides.js";

const { contractName: opcodeTesterName, abi: opcodeTesterAbi, bytecode: opcodeTesterBytecode } = opcodeTesterJson;
const {
  contractName: yulBasedOpcodeTestingName,
  abi: yulBasedOpcodeTestingAbi,
  bytecode: yulBasedOpcodeTestingBytecode,
} = yulBasedOpcodeTestingJson;

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);

  console.log(`Deploying EVM Yul based contract with verbatim bytecode`);
  const yulBasedOpcodeTestingAddress = await deployyulBasedOpcodeTesting(wallet);

  console.log(`Deploying the main OPCODE tester with yul contract at ${yulBasedOpcodeTestingAddress}`);
  await deployOpcodeTester(wallet, yulBasedOpcodeTestingAddress);
}

async function deployyulBasedOpcodeTesting(wallet: ethers.Wallet): Promise<string> {
  const walletNonce = await wallet.getNonce();

  const yulBasedOpcodeTesting = await deployContractFromArtifacts(
    yulBasedOpcodeTestingName,
    yulBasedOpcodeTestingAbi,
    yulBasedOpcodeTestingBytecode,
    wallet,
    {
      nonce: walletNonce,
      ...LOCAL_L2_DEPLOY_FEE_OVERRIDES,
    },
  );

  const yulBasedOpcodeTestingAddress = await yulBasedOpcodeTesting.getAddress();

  return yulBasedOpcodeTestingAddress;
}

async function deployOpcodeTester(wallet: ethers.Wallet, yulBasedOpcodeTestingAddress: string) {
  const walletNonce = await wallet.getNonce();

  await deployContractFromArtifacts(
    opcodeTesterName,
    opcodeTesterAbi,
    opcodeTesterBytecode,
    wallet,
    yulBasedOpcodeTestingAddress,
    {
      nonce: walletNonce,
      ...LOCAL_L2_DEPLOY_FEE_OVERRIDES,
    },
  );
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
