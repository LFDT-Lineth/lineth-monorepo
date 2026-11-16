import { buildL1ProtocolModule } from "../lib/l1Protocol.js";
import { resolvePlonkVerifierShape } from "../lib/plonkVerifier.js";

export default buildL1ProtocolModule(resolvePlonkVerifierShape());
