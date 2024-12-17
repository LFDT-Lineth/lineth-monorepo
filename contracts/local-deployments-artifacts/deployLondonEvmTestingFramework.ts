import { ethers } from "ethers";
import { abi as londonEvmYulAbi, bytecode as londonEvmYulBytecode } from "./static-artifacts/LondonEvmCodes.json";
import { abi as opcodeTesterAbi, bytecode as opcodeTesterBytecode } from "./static-artifacts/OpcodeTester.json";
import { deployContractFromArtifacts } from "../common/helpers/deployments";

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.PRIVATE_KEY!, provider);

  console.log(`Deploying London EVM Yul based contract with verbatim bytecode`);
  const londonEvmYulAddress = await deployLondonEvmYul(wallet, provider);

  console.log(`Deploying the main OPCODE tester with yul contract at ${londonEvmYulAddress}`);
  await deployOpcodeTester(wallet, provider, londonEvmYulAddress);
}

async function deployLondonEvmYul(wallet: ethers.Wallet, provider: ethers.JsonRpcApiProvider): Promise<string> {
  const walletNonce = await wallet.getNonce();

  const londonEvmYul = await deployContractFromArtifacts(londonEvmYulAbi, londonEvmYulBytecode, wallet, {
    nonce: walletNonce,
  });

  const londonEvmYulAddress = await londonEvmYul.getAddress();

  const chainId = (await provider.getNetwork()).chainId;

  console.log(`londonEvmYulAddress deployed: address=${londonEvmYulAddress} chainId=${chainId}`);

  return londonEvmYulAddress;
}

async function deployOpcodeTester(
  wallet: ethers.Wallet,
  provider: ethers.JsonRpcApiProvider,
  londonEvmYulAddress: string,
) {
  const walletNonce = await wallet.getNonce();

  const opcodeTester = await deployContractFromArtifacts(
    opcodeTesterAbi,
    opcodeTesterBytecode,
    wallet,
    londonEvmYulAddress,
    {
      nonce: walletNonce,
    },
  );

  const opcodeTesterAddress = await opcodeTester.getAddress();

  const chainId = (await provider.getNetwork()).chainId;

  console.log(`opcodeTesterAddress deployed: address=${opcodeTesterAddress} chainId=${chainId}`);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
