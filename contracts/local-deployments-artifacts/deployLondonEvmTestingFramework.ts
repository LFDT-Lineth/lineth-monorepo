import { ethers } from "ethers";
import {
  contractName as londonEvmYulName,
  abi as londonEvmYulAbi,
  bytecode as londonEvmYulBytecode,
} from "./static-artifacts/LondonEvmCodes.json";
import {
  contractName as opcodeTesterName,
  abi as opcodeTesterAbi,
  bytecode as opcodeTesterBytecode,
} from "./static-artifacts/OpcodeTester.json";
import { deployContractFromArtifacts } from "../common/helpers/deployments";

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.PRIVATE_KEY!, provider);

  console.log(`Deploying London EVM Yul based contract with verbatim bytecode`);
  const londonEvmYulAddress = await deployLondonEvmYul(wallet);

  console.log(`Deploying the main OPCODE tester with yul contract at ${londonEvmYulAddress}`);
  await deployOpcodeTester(wallet, londonEvmYulAddress);
}

async function deployLondonEvmYul(wallet: ethers.Wallet): Promise<string> {
  const walletNonce = await wallet.getNonce();

  const londonEvmYul = await deployContractFromArtifacts(
    londonEvmYulName,
    londonEvmYulAbi,
    londonEvmYulBytecode,
    wallet,
    {
      nonce: walletNonce,
    },
  );

  const londonEvmYulAddress = await londonEvmYul.getAddress();

  return londonEvmYulAddress;
}

async function deployOpcodeTester(wallet: ethers.Wallet, londonEvmYulAddress: string) {
  const walletNonce = await wallet.getNonce();

  await deployContractFromArtifacts(
    opcodeTesterName,
    opcodeTesterAbi,
    opcodeTesterBytecode,
    wallet,
    londonEvmYulAddress,
    {
      nonce: walletNonce,
    },
  );
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
