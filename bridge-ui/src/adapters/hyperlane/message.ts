import { BridgeMessage, HyperlaneBridgeMessage } from "@/types";

export function isHyperlaneBridgeMessage(msg: BridgeMessage): msg is HyperlaneBridgeMessage {
  return (
    typeof (msg as HyperlaneBridgeMessage).messageId === "string" &&
    typeof (msg as HyperlaneBridgeMessage).transferIndex === "bigint" &&
    typeof (msg as HyperlaneBridgeMessage).sender === "string" &&
    typeof (msg as HyperlaneBridgeMessage).recipient === "string"
  );
}
