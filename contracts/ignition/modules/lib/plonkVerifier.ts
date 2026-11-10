import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { getOptionalEnvVar, getRequiredEnvVar } from "../../../common/helpers/environment.js";

import type { IgnitionModuleBuilder } from "@nomicfoundation/ignition-core";

/**
 * Structural choices for the verifier deployment. Ignition parameters cannot pick which contract to deploy or whether a
 * future exists, so these two are read when the module is loaded (from the same env vars as the Hardhat 2 script);
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

/** Adds `Mimc` (deployed or reused) and the Mimc-linked verifier to `m`. Parameters: `verifierChainConfiguration`, `mimcLibrary`. */
export function addPlonkVerifier(m: IgnitionModuleBuilder, shape: PlonkVerifierShape) {
  const mimc = shape.reuseMimc
    ? m.contractAt("Mimc", m.getParameter<string>("mimcLibrary"), { id: "Mimc" })
    : m.library("Mimc", { id: "Mimc" });
  const verifier = m.contract(shape.verifierContractName, [m.getParameter("verifierChainConfiguration")], {
    id: "PlonkVerifier",
    libraries: { Mimc: mimc },
  });
  return { verifier, mimc };
}

const modulesByShape = new Map<string, ReturnType<typeof createPlonkVerifierModule>>();

function createPlonkVerifierModule(shape: PlonkVerifierShape) {
  return buildModule("PlonkVerifier", (m) => addPlonkVerifier(m, shape));
}

/** One module object per shape, so composites and the standalone module never hold two different "PlonkVerifier" modules. */
export function buildPlonkVerifierModule(shape: PlonkVerifierShape) {
  const key = `${shape.verifierContractName}|${shape.reuseMimc}`;
  const existing = modulesByShape.get(key);
  if (existing !== undefined) {
    return existing;
  }
  const created = createPlonkVerifierModule(shape);
  modulesByShape.set(key, created);
  return created;
}
