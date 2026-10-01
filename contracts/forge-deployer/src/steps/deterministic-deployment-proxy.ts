// Installs the EIP-7997 / Arachnid deterministic deployment proxy on L2.
//
// Thin glue over contracts/common/helpers/deterministicDeploymentProxy.ts.
// The factory lands at a well-known constant address rather than a
// nonce-derived CREATE address, so this step records the deployment via the
// shared parent IPC channel instead of deployContractFromArtifacts.
import { ethers } from "ethers";

import { awaitParentCheckpoint, awaitParentDeploymentIntent } from "../../../common/helpers/deploymentRecord";
import { ensureAndDescribeDeterministicDeploymentProxy } from "../../../common/helpers/deterministicDeploymentProxy";
import { getRequiredEnvVar } from "../../../common/helpers/environment";
import { resolveL2DeployFeeOverrides } from "../../../common/helpers/feeOverrides";

const CONTRACT_NAME = "DeterministicDeploymentProxy";

async function main(): Promise<void> {
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(getRequiredEnvVar("DEPLOYER_PRIVATE_KEY"), provider);
  const rawNonce = getRequiredEnvVar("L2_NONCE");
  if (!/^[0-9]+$/.test(rawNonce)) {
    throw new Error(`L2_NONCE must be a non-negative integer, got: ${rawNonce}`);
  }
  const nonce = Number(rawNonce);
  const fees = resolveL2DeployFeeOverrides();

  await awaitParentDeploymentIntent(CONTRACT_NAME);
  // On an early skip the factory already existed; the record reuses the
  // current head as a stable block reference. On a real deploy it uses the
  // broadcast tx's block (see ensureAndDescribeDeterministicDeploymentProxy).
  const { record, formattedRecord } = await ensureAndDescribeDeterministicDeploymentProxy({
    provider,
    wallet,
    fees,
    nonce,
  });
  await awaitParentCheckpoint({ ...record, chainId: record.chainId.toString() });
  console.log(formattedRecord);
}

main().catch((error: unknown) => {
  console.error(error);
  process.exitCode = 1;
});
