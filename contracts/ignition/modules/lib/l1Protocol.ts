import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { buildPlonkVerifierModule } from "./plonkVerifier.js";
import TimelockModule from "../governance/Timelock.js";
import { addLinethRollup } from "../rollup/LinethRollup.js";

import type { PlonkVerifierShape } from "./plonkVerifier.js";

/**
 * PlonkVerifier → LinethRollup (with the new verifier as `defaultVerifier`) → Timelock.
 * Parameters: `PlonkVerifier.*`, `L1Protocol.*` (the LinethRollup parameters except `verifier`), `Timelock.*`.
 */
export function buildL1ProtocolModule(shape: PlonkVerifierShape) {
  return buildModule("L1Protocol", (m) => {
    const { verifier, mimc } = m.useModule(buildPlonkVerifierModule(shape));
    const rollup = addLinethRollup(m, verifier);
    const { timelock } = m.useModule(TimelockModule);
    return { verifier, mimc, ...rollup, timelock };
  });
}
