import { BridgeMessage, CctpV2BridgeMessage } from "@/types";

export function isCctpV2BridgeMessage(msg: BridgeMessage): msg is CctpV2BridgeMessage {
  return (
    typeof (msg as CctpV2BridgeMessage).nonce === "string" &&
    typeof (msg as CctpV2BridgeMessage).message === "string" &&
    typeof (msg as CctpV2BridgeMessage).attestation === "string"
  );
}
