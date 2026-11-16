import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { getOptionalEnvVar, getRequiredEnvVar } from "../../../common/helpers/environment.js";

/**
 * Structural choices for the verifier deployment. Ignition parameters cannot pick which contract to deploy or whether a
 * future exists, so these two are read from env vars when the module is loaded;
 * every value goes through `m.getParameter`.
 */
export type PlonkVerifierShape = {
  /** Contract name or fully qualified name (`src/...sol:Name`). */
  verifierContractName: string;
  /** `true` → link the existing library from the `mimcLibrary` parameter; `false` → deploy a new `Mimc`. */
  reuseMimc: boolean;
};

/** `VERIFIER_CONTRACT_NAME` (required) and `VERIFIER_MIMC_ADDRESS` (optional, non-empty → reuse). */
export function resolvePlonkVerifierShape(): PlonkVerifierShape {
  return {
    verifierContractName: getRequiredEnvVar("VERIFIER_CONTRACT_NAME"),
    reuseMimc: Boolean(getOptionalEnvVar("VERIFIER_MIMC_ADDRESS")?.trim()),
  };
}

/** `Mimc` (deployed or reused) and the Mimc-linked verifier. Parameters: `verifierChainConfiguration`, `mimcLibrary`. */
export function buildPlonkVerifierModule(shape: PlonkVerifierShape) {
  return buildModule("PlonkVerifier", (m) => {
    const mimc = shape.reuseMimc
      ? m.contractAt("Mimc", m.getParameter<string>("mimcLibrary"), { id: "Mimc" })
      : m.library("Mimc", { id: "Mimc" });
    const verifier = m.contract(shape.verifierContractName, [m.getParameter("verifierChainConfiguration")], {
      id: "PlonkVerifier",
      libraries: { Mimc: mimc },
    });
    return { verifier, mimc };
  });
}
