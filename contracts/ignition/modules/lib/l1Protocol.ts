import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { buildPlonkVerifierModule } from "./plonkVerifier.js";
import TimelockModule from "../governance/Timelock.js";
import { addLinethRollup } from "../rollup/LinethRollup.js";

import type { PlonkVerifierShape } from "./plonkVerifier.js";

const modulesByShape = new Map<string, ReturnType<typeof createL1ProtocolModule>>();

function createL1ProtocolModule(shape: PlonkVerifierShape) {
  return buildModule("L1Protocol", (m) => {
    const { verifier, mimc } = m.useModule(buildPlonkVerifierModule(shape));
    const rollup = addLinethRollup(m, verifier);
    const { timelock } = m.useModule(TimelockModule);
    return { verifier, mimc, ...rollup, timelock };
  });
}

/**
 * PlonkVerifier → LinethRollup (with the new verifier as `defaultVerifier`) → Timelock, replacing
 * `hardhat deploy --tags PlonkVerifier,LinethRollup,Timelock` and its in-process address handoff.
 * Parameters: `PlonkVerifier.*`, `L1Protocol.*` (the LinethRollup parameters except `verifier`), `Timelock.*`.
 */
export function buildL1ProtocolModule(shape: PlonkVerifierShape) {
  const key = `${shape.verifierContractName}|${shape.reuseMimc}`;
  const existing = modulesByShape.get(key);
  if (existing !== undefined) {
    return existing;
  }
  const created = createL1ProtocolModule(shape);
  modulesByShape.set(key, created);
  return created;
}
