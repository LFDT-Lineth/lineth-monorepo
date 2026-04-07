// The current supported EVM version is Osaka, which is a hardfork of the EVM.
import { ethers } from "ethers";

import {
  contractName as opcodeTesterName,
  abi as opcodeTesterAbi,
  bytecode as opcodeTesterBytecode,
} from "./static-artifacts/OpcodeTester.json";
import {
  contractName as yulBasedOpcodeTestingName,
  abi as yulBasedOpcodeTestingAbi,
  bytecode as yulBasedOpcodeTestingBytecode,
} from "./static-artifacts/YulBasedOpcodeTesting.json";
import { deployContractFromArtifacts } from "../common/helpers/deployments";

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
      maxFeePerGas: 7_200_000_000_000n,
      maxPriorityFeePerGas: 7_000_000_000_000n,
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
      maxFeePerGas: 7_200_000_000_000n,
      maxPriorityFeePerGas: 7_000_000_000_000n,
    },
  );
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
