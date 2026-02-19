import { ethers } from "ethers";
import {
  contractName as TestERC20ContractName,
  abi as TestERC20Abi,
  bytecode as TestERC20Bytecode,
} from "./static-artifacts/TestERC20.json";
import { deployContractFromArtifacts } from "../common/helpers/deployments";
import { get1559Fees } from "../scripts/utils";
import { getRequiredEnvVar } from "../common/helpers/environment";

async function main() {
  const ORDERED_NONCE_POST_LINEAROLLUP = 4;
  const ORDERED_NONCE_POST_TOKENBRIDGE = 5;
  const ORDERED_NONCE_POST_L2MESSAGESERVICE = 3;

  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);

  const erc20Name = getRequiredEnvVar("TEST_ERC20_NAME");
  const erc20Symbol = getRequiredEnvVar("TEST_ERC20_SYMBOL");
  const erc20Supply = getRequiredEnvVar("TEST_ERC20_INITIAL_SUPPLY");

  let walletNonce;
  let fees = {};

  if (process.env.TEST_ERC20_L1 === "true") {
    if (!process.env.L1_NONCE) {
      walletNonce = await wallet.getNonce();
    } else {
      walletNonce = parseInt(process.env.L1_NONCE) + ORDERED_NONCE_POST_LINEAROLLUP + ORDERED_NONCE_POST_TOKENBRIDGE;
    }
    fees = { gasPrice: (await get1559Fees(provider)).gasPrice };
  } else {
    if (!process.env.L2_NONCE) {
      walletNonce = await wallet.getNonce();
    } else {
      walletNonce =
        parseInt(process.env.L2_NONCE) + ORDERED_NONCE_POST_L2MESSAGESERVICE + ORDERED_NONCE_POST_TOKENBRIDGE;
    }
    fees = {
      maxFeePerGas: 7_200_000_000_000n,
      maxPriorityFeePerGas: 7_000_000_000_000n,
    };
  }

  await deployContractFromArtifacts(
    TestERC20ContractName,
    TestERC20Abi,
    TestERC20Bytecode,
    wallet,
    erc20Name,
    erc20Symbol,
    erc20Supply,
    {
      nonce: walletNonce,
      ...fees,
    },
  );
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
