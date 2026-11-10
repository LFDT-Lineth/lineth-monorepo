import { concat, encodeAbiParameters, parseAbiParameters } from "viem";
import { getAddress, isAddress } from "viem";

import { getOptionalEnvVar, getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { formatEnvVarValueForMessage } from "../../../common/helpers/envVarLogging.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { toBytes32, toUint } from "../lib/parameters.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `01_deploy_PlonkVerifier.ts`. Also validates `VERIFIER_PROOF_TYPE` (used for the printed `setVerifierAddress` calldata). */
export async function plonkVerifierParameters(context: ParameterContext): Promise<ModuleParameters> {
  const { networkName } = context;
  getRequiredEnvVar("VERIFIER_CONTRACT_NAME");
  toUint("VERIFIER_PROOF_TYPE", getRequiredEnvVar("VERIFIER_PROOF_TYPE"));
  const chainId = getRequiredEnvVar("VERIFIER_CHAIN_ID");
  const baseFee = getRequiredEnvVar("VERIFIER_BASE_FEE");
  const coinbase = getRequiredEnvVar("VERIFIER_COINBASE");
  const l2MessageServiceAddress = requireAddressFromRegistryOrEnv(
    networkName,
    "L2MessageService",
    "L2_MESSAGE_SERVICE_ADDRESS",
  );
  const isAllowedCircuitId = getRequiredEnvVar("VERIFIER_IS_ALLOWED_CIRCUIT_ID");

  const parameters: ModuleParameters = {
    verifierChainConfiguration: [
      { value: toBytes32("VERIFIER_CHAIN_ID", chainId), name: "chainId" },
      { value: toBytes32("VERIFIER_BASE_FEE", baseFee), name: "baseFee" },
      { value: toBytes32("VERIFIER_COINBASE", coinbase), name: "coinbase" },
      { value: toBytes32("L2_MESSAGE_SERVICE_ADDRESS", l2MessageServiceAddress), name: "l2MessageServiceAddress" },
      { value: toBytes32("VERIFIER_IS_ALLOWED_CIRCUIT_ID", isAllowedCircuitId), name: "isAllowedCircuitId" },
    ],
  };

  const optionalMimcAddress = getOptionalEnvVar("VERIFIER_MIMC_ADDRESS")?.trim();
  if (optionalMimcAddress) {
    if (!isAddress(optionalMimcAddress)) {
      throw new Error(
        `VERIFIER_MIMC_ADDRESS must be a valid address, got "${formatEnvVarValueForMessage("VERIFIER_MIMC_ADDRESS", optionalMimcAddress)}"`,
      );
    }
    const mimcAddress = getAddress(optionalMimcAddress);
    const code = await context.getCode(mimcAddress);
    if (code === undefined || code === "0x") {
      throw new Error(
        `VERIFIER_MIMC_ADDRESS ${mimcAddress} has no contract bytecode on this network; deploy Mimc first or unset VERIFIER_MIMC_ADDRESS to deploy a new library.`,
      );
    }
    console.log(`Reusing existing Mimc library at ${mimcAddress} (VERIFIER_MIMC_ADDRESS)`);
    parameters.mimcLibrary = mimcAddress;
  }

  return parameters;
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  PlonkVerifier: await plonkVerifierParameters(context),
});

/** Prints the `setVerifierAddress(address,uint256)` calldata (selector 0xc2116974) for the new verifier. */
export const afterDeploy: NonNullable<DeployCompanion["afterDeploy"]> = ({ contracts }) => {
  const verifier = contracts["PlonkVerifier#PlonkVerifier"];
  if (verifier === undefined) {
    throw new Error('Future "PlonkVerifier#PlonkVerifier" is not in the deployment result.');
  }
  const verifierIndex = toUint("VERIFIER_PROOF_TYPE", getRequiredEnvVar("VERIFIER_PROOF_TYPE"));
  const setVerifierAddress = concat([
    "0xc2116974",
    encodeAbiParameters(parseAbiParameters("address, uint256"), [verifier, verifierIndex]),
  ]);
  console.log("setVerifierAddress calldata:", setVerifierAddress);
};
