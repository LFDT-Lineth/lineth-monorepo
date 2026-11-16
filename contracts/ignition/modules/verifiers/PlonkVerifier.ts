import { buildPlonkVerifierModule, resolvePlonkVerifierShape } from "../lib/plonkVerifier.js";

/**
 * `Mimc` (new, or reused from `VERIFIER_MIMC_ADDRESS`) and the
 * `VERIFIER_CONTRACT_NAME` verifier linked against it. Parameters: `PlonkVerifier.deploy.ts`.
 */
export default buildPlonkVerifierModule(resolvePlonkVerifierShape());
