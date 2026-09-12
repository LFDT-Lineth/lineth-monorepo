// Deploys the EIP-7997 / Arachnid deterministic deployment proxy on L2 for the
// lineth-stack quickstart. Thin entrypoint over the shared helper so the same
// fund-then-broadcast logic serves both this scaffold and the forge-deployer.
//
// Emits `contract=DeterministicDeploymentProxy deployed: address=...` so the
// 04-deploy-contracts.sh extract_address/verify_address helpers work unchanged.
import { ensureAndDescribeDeterministicDeploymentProxy } from "contracts/common/helpers/deterministicDeploymentProxy";
import { resolveL2DeployFeeOverrides } from "contracts/common/helpers/feeOverrides";
import { JsonRpcProvider, Wallet } from "ethers";

import { requiredProcessEnv } from "./lib/env";
import { sanitizeExternalError } from "./lib/errors";

async function main() {
  const rpcUrl = requiredProcessEnv("L2_RPC_URL");
  const provider = new JsonRpcProvider(rpcUrl);
  const wallet = new Wallet(requiredProcessEnv("L2_DEPLOYER_PRIVATE_KEY"), provider);
  const fees = resolveL2DeployFeeOverrides();
  const nonce = await provider.getTransactionCount(wallet.address, "pending");

  const { formattedRecord } = await ensureAndDescribeDeterministicDeploymentProxy({ provider, wallet, fees, nonce });
  console.log(formattedRecord);
  provider.destroy();
}

main().catch((error: unknown) => {
  process.stderr.write(`${sanitizeExternalError(error)}\n`);
  process.exit(1);
});
