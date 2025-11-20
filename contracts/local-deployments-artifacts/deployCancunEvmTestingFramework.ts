import { ethers } from "ethers";
import {
  contractName as shanghaiEvmYulName,
  abi as shanghaiEvmYulAbi,
  bytecode as shanghaiEvmYulBytecode,
} from "./static-artifacts/ShanghaiEvmCodes.json";
import {
  contractName as opcodeTesterName,
  abi as opcodeTesterAbi,
  bytecode as opcodeTesterBytecode,
} from "./static-artifacts/OpcodeTester.json";
import { deployContractFromArtifacts } from "../common/helpers/deployments";

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.PRIVATE_KEY!, provider);

  console.log(`Deploying Shanghai EVM Yul based contract with verbatim bytecode`);
  const shanghaiEvmYulAddress = await deployShanghaiEvmYul(wallet);

  console.log(`Deploying the main OPCODE tester with yul contract at ${shanghaiEvmYulAddress}`);
  await deployOpcodeTester(wallet, shanghaiEvmYulAddress);
}

async function deployShanghaiEvmYul(wallet: ethers.Wallet): Promise<string> {
  const walletNonce = await wallet.getNonce();

  const shanghaiEvmYul = await deployContractFromArtifacts(
    shanghaiEvmYulName,
    shanghaiEvmYulAbi,
    shanghaiEvmYulBytecode,
    wallet,
    {
      nonce: walletNonce,
      maxFeePerGas: 7_200_000_000_000n,
      maxPriorityFeePerGas: 7_000_000_000_000n,
    },
  );

  const shanghaiEvmYulAddress = await shanghaiEvmYul.getAddress();

  return shanghaiEvmYulAddress;
}

async function deployOpcodeTester(wallet: ethers.Wallet, shanghaiEvmYulAddress: string) {
  const walletNonce = await wallet.getNonce();

  await deployContractFromArtifacts(
    opcodeTesterName,
    opcodeTesterAbi,
    opcodeTesterBytecode,
    wallet,
    shanghaiEvmYulAddress,
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
