import * as dotenv from "dotenv";
import { ethers } from "ethers";

import { ensureAndDescribeDeterministicDeploymentProxy } from "../common/helpers/deterministicDeploymentProxy";
import { getRequiredEnvVar } from "../common/helpers/environment";
import { resolveL2DeployFeeOverrides } from "../common/helpers/feeOverrides";

dotenv.config();

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(getRequiredEnvVar("DEPLOYER_PRIVATE_KEY"), provider);
  const fees = resolveL2DeployFeeOverrides();
  const nonce = await provider.getTransactionCount(wallet.address, "pending");

  const { formattedRecord } = await ensureAndDescribeDeterministicDeploymentProxy({ provider, wallet, fees, nonce });
  console.log(formattedRecord);
}

main().catch((error: unknown) => {
  console.error(error);
  process.exitCode = 1;
});
