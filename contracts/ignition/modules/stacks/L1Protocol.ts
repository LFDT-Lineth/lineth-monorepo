import { buildL1ProtocolModule } from "../lib/l1Protocol.js";
import { resolvePlonkVerifierShape } from "../lib/plonkVerifier.js";

/** Replaces `hardhat deploy --tags PlonkVerifier,LinethRollup,Timelock`. */
export default buildL1ProtocolModule(resolvePlonkVerifierShape());
