import { buildPlonkVerifierModule, resolvePlonkVerifierShape } from "../lib/plonkVerifier.js";

/**
 * Replaces `01_deploy_PlonkVerifier.ts` (tag `PlonkVerifier`): `Mimc` (new, or reused from `VERIFIER_MIMC_ADDRESS`) and the
 * `VERIFIER_CONTRACT_NAME` verifier linked against it. Parameters: `PlonkVerifier.deploy.ts`.
 */
export default buildPlonkVerifierModule(resolvePlonkVerifierShape());
